/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package controller

import (
	"context"

	surrealdbv1alpha1 "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	operatorconditions "github.com/Seamlezz/surrealdb-credential-operator/internal/conditions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("SurrealDBTenantPolicy Controller", func() {
	ctx := context.Background()

	It("marks tenant policy ready when provider exists", func() {
		ns := "policy-ready"
		createIgnoringAlreadyExists(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		provider := &surrealdbv1alpha1.SurrealDBProvider{ObjectMeta: metav1.ObjectMeta{Name: "provider-policy-ready"}, Spec: surrealdbv1alpha1.SurrealDBProviderSpec{Endpoint: "ws://surrealdb:8000", RootCredentialRef: surrealdbv1alpha1.RootCredentialReference{Namespace: ns, Name: "root"}}}
		Expect(k8sClient.Create(ctx, provider)).To(Succeed())
		policy := &surrealdbv1alpha1.SurrealDBTenantPolicy{ObjectMeta: metav1.ObjectMeta{Name: "smoke", Namespace: ns}, Spec: surrealdbv1alpha1.SurrealDBTenantPolicySpec{ProviderRef: surrealdbv1alpha1.LocalProviderReference{Name: provider.Name}, SurrealNamespace: "smoke"}}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		latest := reconcileTenantPolicy(ctx, types.NamespacedName{Namespace: ns, Name: policy.Name})

		Expect(latest.Status.ProviderRef).NotTo(BeNil())
		Expect(latest.Status.ProviderRef.Name).To(Equal(provider.Name))
		ready := meta.FindStatusCondition(latest.Status.Conditions, operatorconditions.TypeReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal(operatorconditions.ReasonAvailable))
	})

	It("marks tenant policy not ready when provider is missing", func() {
		ns := "policy-missing-provider"
		createIgnoringAlreadyExists(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		policy := &surrealdbv1alpha1.SurrealDBTenantPolicy{ObjectMeta: metav1.ObjectMeta{Name: "smoke", Namespace: ns}, Spec: surrealdbv1alpha1.SurrealDBTenantPolicySpec{ProviderRef: surrealdbv1alpha1.LocalProviderReference{Name: "missing-provider"}, SurrealNamespace: "smoke"}}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		latest := reconcileTenantPolicy(ctx, types.NamespacedName{Namespace: ns, Name: policy.Name})

		Expect(latest.Status.ProviderRef).NotTo(BeNil())
		Expect(latest.Status.ProviderRef.Name).To(Equal("missing-provider"))
		ready := meta.FindStatusCondition(latest.Status.Conditions, operatorconditions.TypeReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(operatorconditions.ReasonProviderNotFound))
	})
})

func reconcileTenantPolicy(ctx context.Context, name types.NamespacedName) *surrealdbv1alpha1.SurrealDBTenantPolicy {
	reconciler := &SurrealDBTenantPolicyReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: name})
	Expect(err).NotTo(HaveOccurred())
	latest := &surrealdbv1alpha1.SurrealDBTenantPolicy{}
	Expect(k8sClient.Get(ctx, name, latest)).To(Succeed())
	return latest
}
