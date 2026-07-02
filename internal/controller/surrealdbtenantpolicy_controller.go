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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// SurrealDBTenantPolicyReconciler reconciles a SurrealDBTenantPolicy object.
type SurrealDBTenantPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbtenantpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbtenantpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbtenantpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=surrealdb.seamlezz.com,resources=surrealdbproviders,verbs=get;list;watch

// Reconcile validates tenant policy references and reports status.
func (r *SurrealDBTenantPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	policy := &surrealdbv1alpha1.SurrealDBTenantPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	policy.Status.ObservedGeneration = policy.Generation
	policy.Status.ProviderRef = &surrealdbv1alpha1.LocalProviderReference{Name: policy.Spec.ProviderRef.Name}

	provider := &surrealdbv1alpha1.SurrealDBProvider{}
	if err := r.Get(ctx, types.NamespacedName{Name: policy.Spec.ProviderRef.Name}, provider); err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("referenced SurrealDBProvider %q not found", policy.Spec.ProviderRef.Name)
			operatorconditions.NotReady(&policy.Status.Conditions, policy.Generation, operatorconditions.ReasonProviderNotFound, msg)
			logger.Info("Tenant policy provider not found", "policy", req.NamespacedName, "provider", policy.Spec.ProviderRef.Name)
			return ctrl.Result{}, r.Status().Update(ctx, policy)
		}
		return ctrl.Result{}, err
	}

	operatorconditions.Ready(&policy.Status.Conditions, policy.Generation, operatorconditions.ReasonAvailable, "tenant policy references an existing provider")
	if err := r.Status().Update(ctx, policy); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SurrealDBTenantPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&surrealdbv1alpha1.SurrealDBTenantPolicy{}).
		Named("surrealdbtenantpolicy").
		Complete(r)
}
