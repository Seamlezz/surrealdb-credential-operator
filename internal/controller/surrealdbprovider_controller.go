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
	"fmt"

	surrealdbv1alpha1 "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	operatorconditions "github.com/Seamlezz/surrealdb-credential-operator/internal/conditions"
	"github.com/Seamlezz/surrealdb-credential-operator/internal/surreal"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// SurrealDBProviderReconciler reconciles a SurrealDBProvider object.
type SurrealDBProviderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbproviders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbproviders/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates provider Secret/TLS references and reports status.
func (r *SurrealDBProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	provider := &surrealdbv1alpha1.SurrealDBProvider{}
	if err := r.Get(ctx, req.NamespacedName, provider); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	provider.Status.ObservedGeneration = provider.Generation
	if err := r.validateProvider(ctx, provider); err != nil {
		operatorconditions.NotReady(&provider.Status.Conditions, provider.Generation, operatorconditions.ReasonProviderSecretInvalid, err.Error())
		logger.Info("Provider validation failed", "provider", provider.Name, "error", err)
	} else {
		operatorconditions.Ready(&provider.Status.Conditions, provider.Generation, operatorconditions.ReasonAvailable, "provider references are valid")
	}

	if err := r.Status().Update(ctx, provider); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *SurrealDBProviderReconciler) validateProvider(ctx context.Context, provider *surrealdbv1alpha1.SurrealDBProvider) error {
	secret := &corev1.Secret{}
	ref := provider.Spec.RootCredentialRef
	if err := r.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("root credential Secret %s/%s not found", ref.Namespace, ref.Name)
		}
		return fmt.Errorf("get root credential Secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}
	usernameKey := ref.UsernameKey
	if usernameKey == "" {
		usernameKey = "username"
	}
	passwordKey := ref.PasswordKey
	if passwordKey == "" {
		passwordKey = "password"
	}
	if len(secret.Data[usernameKey]) == 0 {
		return fmt.Errorf("root credential Secret %s/%s missing key %q", ref.Namespace, ref.Name, usernameKey)
	}
	if len(secret.Data[passwordKey]) == 0 {
		return fmt.Errorf("root credential Secret %s/%s missing key %q", ref.Namespace, ref.Name, passwordKey)
	}
	if _, err := surreal.LoadTLSConfig(ctx, r.Client, provider.Spec.TLS); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SurrealDBProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&surrealdbv1alpha1.SurrealDBProvider{}).
		Named("surrealdbprovider").
		Complete(r)
}
