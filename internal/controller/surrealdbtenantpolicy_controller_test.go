/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package controller

import (
	"context"

	surrealdbv1alpha1 "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("SurrealDBTenantPolicy Controller", func() {
	ctx := context.Background()

	It("marks tenant policy ready when provider exists", func() {
		ns := "policy-ready"
		createIgnoringAlreadyExists(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		provider := &surrealdbv1alpha1.SurrealDBProvider{ObjectMeta: metav1.ObjectMeta{Name: "provider-policy-ready"}, Spec: surrealdbv1alpha1.SurrealDBProviderSpec{Endpoint: "http://surrealdb:8000", RootCredentialRef: surrealdbv1alpha1.RootCredentialReference{Namespace: ns, Name: "root"}}}
		Expect(k8sClient.Create(ctx, provider)).To(Succeed())
		policy := &surrealdbv1alpha1.SurrealDBTenantPolicy{ObjectMeta: metav1.ObjectMeta{Name: "smoke", Namespace: ns}, Spec: surrealdbv1alpha1.SurrealDBTenantPolicySpec{ProviderRef: surrealdbv1alpha1.LocalProviderReference{Name: provider.Name}, SurrealNamespace: "smoke"}}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		reconciler := &SurrealDBTenantPolicyReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: policy.Name}})
		Expect(err).NotTo(HaveOccurred())

		latest := &surrealdbv1alpha1.SurrealDBTenantPolicy{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: policy.Name}, latest)).To(Succeed())
		Expect(latest.Status.Conditions).NotTo(BeEmpty())
	})
})
