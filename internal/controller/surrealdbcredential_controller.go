/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	surrealdbv1alpha1 "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	operatorconditions "github.com/Seamlezz/surrealdb-credential-operator/internal/conditions"
	"github.com/Seamlezz/surrealdb-credential-operator/internal/policy"
	"github.com/Seamlezz/surrealdb-credential-operator/internal/rotation"
	secretutil "github.com/Seamlezz/surrealdb-credential-operator/internal/secrets"
	"github.com/Seamlezz/surrealdb-credential-operator/internal/surreal"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	credentialFinalizer    = "surrealdb.seamlezz.com/credential-finalizer"
	annotationForceCleanup = "surrealdb.seamlezz.com/force-cleanup"
)

var surrealDBUnavailableRequeue = ctrl.Result{Requeue: true}

// AdminFactory creates a SurrealDB admin client.
type AdminFactory func(ctx context.Context, endpoint, username, password string, tlsConfig *tls.Config) (surreal.Admin, error)

// SurrealDBCredentialReconciler reconciles a SurrealDBCredential object.
type SurrealDBCredentialReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	AdminFactory AdminFactory
	Clock        func() time.Time
}

// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbcredentials,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbcredentials/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbcredentials/finalizers,verbs=update
// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbtenantpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbproviders,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile creates, updates, rotates, and deletes generated SurrealDB credentials.
func (r *SurrealDBCredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	credential := &surrealdbv1alpha1.SurrealDBCredential{}
	if err := r.Get(ctx, req.NamespacedName, credential); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if credential.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(credential, credentialFinalizer) {
		controllerutil.AddFinalizer(credential, credentialFinalizer)
		return ctrl.Result{}, r.Update(ctx, credential)
	}

	if !credential.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, credential)
	}

	resolved, ok, err := r.resolve(ctx, credential)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ok {
		return ctrl.Result{}, r.Status().Update(ctx, credential)
	}

	credential.Status.ProviderRef = &surrealdbv1alpha1.LocalProviderReference{Name: resolved.Policy.Spec.ProviderRef.Name}
	credential.Status.SurrealNamespace = resolved.Policy.Spec.SurrealNamespace
	credential.Status.Database = credential.Spec.Database
	credential.Status.Username = resolved.Username
	credential.Status.ObservedGeneration = credential.Generation
	operatorconditions.Set(&credential.Status.Conditions, credential.Generation, operatorconditions.TypePolicyAccepted, metav1.ConditionTrue, operatorconditions.ReasonPolicyAccepted, "credential request is allowed by tenant policy")

	existingSecret, err := r.getTargetSecret(ctx, credential)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := secretutil.EnsureCanManage(existingSecret, credential); err != nil {
		operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonSecretConflict, err.Error())
		operatorconditions.Set(&credential.Status.Conditions, credential.Generation, operatorconditions.TypeSecretReady, metav1.ConditionFalse, operatorconditions.ReasonSecretConflict, err.Error())
		return ctrl.Result{}, r.Status().Update(ctx, credential)
	}

	existingPassword, hasPassword := secretutil.ExistingPassword(existingSecret)
	now := r.now()
	decision := rotation.Decide(now, hasPassword, credential.Annotations, credential.Status.LastManualRotationTrigger, credential.Status.LastRotationTime, credential.Status.NextRotationTime, rotationPeriod(credential))
	password := existingPassword
	if decision.Action == rotation.ActionGenerate || decision.Action == rotation.ActionRotate {
		password, err = secretutil.GeneratePassword(secretutil.DefaultPasswordLength)
		if err != nil {
			operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonRotationFailed, err.Error())
			return ctrl.Result{}, r.Status().Update(ctx, credential)
		}
	}

	admin, err := r.newAdmin(ctx, resolved.Provider, resolved.RootUsername, resolved.RootPassword, resolved.TLSConfig)
	if err != nil {
		operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonSurrealDBUnavailable, err.Error())
		if updateErr := r.Status().Update(ctx, credential); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		return surrealDBUnavailableRequeue, nil
	}
	defer func() { _ = admin.Close(ctx) }()

	if err := admin.DefineUser(ctx, resolved.SurrealTarget, resolved.Username, password, credential.Spec.Roles); err != nil {
		operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonDefineUserFailed, err.Error())
		operatorconditions.Set(&credential.Status.Conditions, credential.Generation, operatorconditions.TypeUserDefined, metav1.ConditionFalse, operatorconditions.ReasonDefineUserFailed, err.Error())
		return ctrl.Result{}, r.Status().Update(ctx, credential)
	}
	operatorconditions.Set(&credential.Status.Conditions, credential.Generation, operatorconditions.TypeUserDefined, metav1.ConditionTrue, operatorconditions.ReasonDefineUserSucceeded, "SurrealDB user is defined")

	if err := r.reconcileTargetSecret(ctx, credential, resolved, password, existingSecret); err != nil {
		operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonSecretConflict, err.Error())
		operatorconditions.Set(&credential.Status.Conditions, credential.Generation, operatorconditions.TypeSecretReady, metav1.ConditionFalse, operatorconditions.ReasonSecretConflict, err.Error())
		return ctrl.Result{}, r.Status().Update(ctx, credential)
	}

	operatorconditions.Set(&credential.Status.Conditions, credential.Generation, operatorconditions.TypeSecretReady, metav1.ConditionTrue, operatorconditions.ReasonSecretWritten, "target Secret is ready")
	operatorconditions.Ready(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonReconciled, "SurrealDB credential is ready")
	if decision.Action == rotation.ActionGenerate || decision.Action == rotation.ActionRotate {
		credential.Status.LastRotationTime = decision.LastRotationTime
	}
	credential.Status.LastManualRotationTrigger = decision.ManualTrigger
	credential.Status.NextRotationTime = decision.NextRotationTime

	if err := r.Status().Update(ctx, credential); err != nil {
		return ctrl.Result{}, err
	}
	if credential.Status.NextRotationTime != nil {
		until := time.Until(credential.Status.NextRotationTime.Time)
		if until < 0 {
			until = time.Second
		}
		logger.V(1).Info("Scheduled next credential rotation", "credential", req.NamespacedName, "after", until)
		return ctrl.Result{RequeueAfter: until}, nil
	}
	return ctrl.Result{}, nil
}

type resolvedCredential struct {
	Policy        *surrealdbv1alpha1.SurrealDBTenantPolicy
	Provider      *surrealdbv1alpha1.SurrealDBProvider
	RootUsername  string
	RootPassword  string
	TLSConfig     *tls.Config
	Username      string
	SurrealTarget surreal.UserTarget
}

func (r *SurrealDBCredentialReconciler) resolve(ctx context.Context, credential *surrealdbv1alpha1.SurrealDBCredential) (*resolvedCredential, bool, error) {
	policyObj := &surrealdbv1alpha1.SurrealDBTenantPolicy{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: credential.Namespace, Name: credential.Spec.PolicyRef.Name}, policyObj); err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("referenced SurrealDBTenantPolicy %q not found", credential.Spec.PolicyRef.Name)
			operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonPolicyNotFound, msg)
			credential.Status.ObservedGeneration = credential.Generation
			return nil, false, nil
		}
		return nil, false, err
	}

	evaluation := policy.Evaluate(policyObj.Spec, policy.CredentialRequest{Level: credential.Spec.Level, Database: credential.Spec.Database, Roles: credential.Spec.Roles})
	if !evaluation.Allowed {
		operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonPolicyDenied, evaluation.Message)
		operatorconditions.Set(&credential.Status.Conditions, credential.Generation, operatorconditions.TypePolicyAccepted, metav1.ConditionFalse, operatorconditions.ReasonPolicyDenied, evaluation.Message)
		credential.Status.ObservedGeneration = credential.Generation
		return nil, false, nil
	}

	provider := &surrealdbv1alpha1.SurrealDBProvider{}
	if err := r.Get(ctx, types.NamespacedName{Name: policyObj.Spec.ProviderRef.Name}, provider); err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("referenced SurrealDBProvider %q not found", policyObj.Spec.ProviderRef.Name)
			operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonProviderNotFound, msg)
			credential.Status.ObservedGeneration = credential.Generation
			return nil, false, nil
		}
		return nil, false, err
	}

	rootUsername, rootPassword, ok, err := r.loadRootCredentials(ctx, provider)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, r.Status().Update(ctx, credential)
	}
	tlsConfig, err := surreal.LoadTLSConfig(ctx, r.Client, provider.Spec.TLS)
	if err != nil {
		operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, operatorconditions.ReasonProviderSecretInvalid, err.Error())
		credential.Status.ObservedGeneration = credential.Generation
		return nil, false, r.Status().Update(ctx, credential)
	}

	username := policy.Username(credential.Namespace, credential.Name, evaluation.Target)
	return &resolvedCredential{
		Policy:       policyObj,
		Provider:     provider,
		RootUsername: rootUsername,
		RootPassword: rootPassword,
		TLSConfig:    tlsConfig,
		Username:     username,
		SurrealTarget: surreal.UserTarget{
			Level:     credential.Spec.Level,
			Namespace: policyObj.Spec.SurrealNamespace,
			Database:  credential.Spec.Database,
		},
	}, true, nil
}

func (r *SurrealDBCredentialReconciler) reconcileDelete(ctx context.Context, credential *surrealdbv1alpha1.SurrealDBCredential) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(credential, credentialFinalizer) {
		return ctrl.Result{}, nil
	}
	if credential.Annotations[annotationForceCleanup] == "true" {
		if err := r.deleteOwnedTargetSecret(ctx, credential); err != nil {
			targetSecret := types.NamespacedName{Namespace: credential.Namespace, Name: credential.Spec.TargetSecret.Name}
			msg := fmt.Sprintf("force cleanup could not delete owned target Secret %s: %v; removing finalizer anyway", targetSecret.String(), err)
			logger.Error(err, "Force cleanup could not delete owned target Secret", "credential", client.ObjectKeyFromObject(credential), "targetSecret", targetSecret)
			r.recordDeleteFailure(ctx, credential, operatorconditions.ReasonSecretDeleteFailed, msg, true)
		}
		return r.removeCredentialFinalizer(ctx, credential)
	}

	resolved, ok, err := r.resolve(ctx, credential)
	if err != nil || !ok {
		return ctrl.Result{}, err
	}
	admin, err := r.newAdmin(ctx, resolved.Provider, resolved.RootUsername, resolved.RootPassword, resolved.TLSConfig)
	if err != nil {
		r.recordDeleteFailure(ctx, credential, operatorconditions.ReasonSurrealDBUnavailable, err.Error(), false)
		return surrealDBUnavailableRequeue, nil
	}
	defer func() { _ = admin.Close(ctx) }()
	if err := admin.RemoveUser(ctx, resolved.SurrealTarget, resolved.Username); err != nil {
		r.recordDeleteFailure(ctx, credential, operatorconditions.ReasonRemoveUserFailed, err.Error(), false)
		return ctrl.Result{}, err
	}
	if err := r.deleteOwnedTargetSecret(ctx, credential); err != nil {
		targetSecret := types.NamespacedName{Namespace: credential.Namespace, Name: credential.Spec.TargetSecret.Name}
		msg := fmt.Sprintf("failed to delete owned target Secret %s: %v", targetSecret.String(), err)
		r.recordDeleteFailure(ctx, credential, operatorconditions.ReasonSecretDeleteFailed, msg, true)
		return ctrl.Result{}, err
	}
	return r.removeCredentialFinalizer(ctx, credential)
}

func (r *SurrealDBCredentialReconciler) recordDeleteFailure(ctx context.Context, credential *surrealdbv1alpha1.SurrealDBCredential, reason, message string, markSecret bool) {
	logger := logf.FromContext(ctx)

	credential.Status.ObservedGeneration = credential.Generation
	operatorconditions.NotReady(&credential.Status.Conditions, credential.Generation, reason, message)
	if markSecret {
		operatorconditions.Set(&credential.Status.Conditions, credential.Generation, operatorconditions.TypeSecretReady, metav1.ConditionFalse, reason, message)
	}

	if err := r.Status().Update(ctx, credential); err != nil {
		logger.Error(err, "Failed to update SurrealDBCredential delete failure status", "credential", client.ObjectKeyFromObject(credential), "reason", reason)
	}
}

func (r *SurrealDBCredentialReconciler) removeCredentialFinalizer(ctx context.Context, credential *surrealdbv1alpha1.SurrealDBCredential) (ctrl.Result, error) {
	latest := &surrealdbv1alpha1.SurrealDBCredential{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(credential), latest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !controllerutil.ContainsFinalizer(latest, credentialFinalizer) {
		return ctrl.Result{}, nil
	}
	controllerutil.RemoveFinalizer(latest, credentialFinalizer)
	return ctrl.Result{}, r.Update(ctx, latest)
}

func (r *SurrealDBCredentialReconciler) loadRootCredentials(ctx context.Context, provider *surrealdbv1alpha1.SurrealDBProvider) (string, string, bool, error) {
	ref := provider.Spec.RootCredentialRef
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, secret); err != nil {
		return "", "", false, err
	}
	usernameKey := ref.UsernameKey
	if usernameKey == "" {
		usernameKey = "username"
	}
	passwordKey := ref.PasswordKey
	if passwordKey == "" {
		passwordKey = "password"
	}
	username := string(secret.Data[usernameKey])
	password := string(secret.Data[passwordKey])
	if username == "" || password == "" {
		return "", "", false, fmt.Errorf("root credential Secret %s/%s missing username or password keys", ref.Namespace, ref.Name)
	}
	return username, password, true, nil
}

func (r *SurrealDBCredentialReconciler) getTargetSecret(ctx context.Context, credential *surrealdbv1alpha1.SurrealDBCredential) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: credential.Namespace, Name: credential.Spec.TargetSecret.Name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return secret, nil
}

func (r *SurrealDBCredentialReconciler) reconcileTargetSecret(ctx context.Context, credential *surrealdbv1alpha1.SurrealDBCredential, resolved *resolvedCredential, password string, existing *corev1.Secret) error {
	level := strings.ToLower(string(credential.Spec.Level))
	desired := secretutil.BuildTargetSecret(credential, secretutil.ConnectionData{
		URL:       resolved.Provider.Spec.Endpoint,
		Namespace: resolved.Policy.Spec.SurrealNamespace,
		Database:  credential.Spec.Database,
		Username:  resolved.Username,
		Password:  password,
		Level:     level,
	})
	if existing == nil {
		return r.Create(ctx, desired)
	}
	existing.Labels = desired.Labels
	existing.OwnerReferences = desired.OwnerReferences
	existing.Type = desired.Type
	existing.Data = desired.Data
	return r.Update(ctx, existing)
}

func (r *SurrealDBCredentialReconciler) deleteOwnedTargetSecret(ctx context.Context, credential *surrealdbv1alpha1.SurrealDBCredential) error {
	secret, err := r.getTargetSecret(ctx, credential)
	if err != nil || secret == nil {
		return err
	}
	if !secretutil.IsOwnedByCredential(secret, credential) {
		return nil
	}
	if err := r.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *SurrealDBCredentialReconciler) newAdmin(ctx context.Context, provider *surrealdbv1alpha1.SurrealDBProvider, username, password string, tlsConfig *tls.Config) (surreal.Admin, error) {
	factory := r.AdminFactory
	if factory == nil {
		factory = func(ctx context.Context, endpoint, username, password string, tlsConfig *tls.Config) (surreal.Admin, error) {
			return surreal.NewAdminClientWithTLS(ctx, endpoint, username, password, tlsConfig)
		}
	}
	return factory(ctx, provider.Spec.Endpoint, username, password, tlsConfig)
}

func (r *SurrealDBCredentialReconciler) now() time.Time {
	if r.Clock != nil {
		return r.Clock().UTC()
	}
	return time.Now().UTC()
}

func rotationPeriod(credential *surrealdbv1alpha1.SurrealDBCredential) *metav1.Duration {
	if credential.Spec.Rotation == nil {
		return nil
	}
	return credential.Spec.Rotation.Period
}

// SetupWithManager sets up the controller with the Manager.
func (r *SurrealDBCredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&surrealdbv1alpha1.SurrealDBCredential{}).
		Owns(&corev1.Secret{}).
		Named("surrealdbcredential").
		Complete(r)
}
