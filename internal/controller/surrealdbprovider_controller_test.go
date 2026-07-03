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

var _ = Describe("SurrealDBProvider Controller", func() {
	ctx := context.Background()

	It("marks provider ready when root Secret keys exist", func() {
		ns := "provider-ready"
		createProviderRootSecret(ctx, ns, "root", map[string][]byte{"username": []byte("root"), "password": []byte("rootpass")})
		provider := &surrealdbv1alpha1.SurrealDBProvider{ObjectMeta: metav1.ObjectMeta{Name: "provider-ready"}, Spec: surrealdbv1alpha1.SurrealDBProviderSpec{Endpoint: "ws://surrealdb:8000", RootCredentialRef: surrealdbv1alpha1.RootCredentialReference{Namespace: ns, Name: "root"}}}
		Expect(k8sClient.Create(ctx, provider)).To(Succeed())

		latest := reconcileProvider(ctx, provider.Name)

		ready := meta.FindStatusCondition(latest.Status.Conditions, operatorconditions.TypeReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal(operatorconditions.ReasonAvailable))
	})

	It("marks provider not ready when root Secret is missing", func() {
		ns := "provider-missing-secret"
		createIgnoringAlreadyExists(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		provider := &surrealdbv1alpha1.SurrealDBProvider{ObjectMeta: metav1.ObjectMeta{Name: "provider-missing-secret"}, Spec: surrealdbv1alpha1.SurrealDBProviderSpec{Endpoint: "ws://surrealdb:8000", RootCredentialRef: surrealdbv1alpha1.RootCredentialReference{Namespace: ns, Name: "missing"}}}
		Expect(k8sClient.Create(ctx, provider)).To(Succeed())

		latest := reconcileProvider(ctx, provider.Name)

		ready := meta.FindStatusCondition(latest.Status.Conditions, operatorconditions.TypeReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(operatorconditions.ReasonProviderSecretInvalid))
	})

	It("marks provider not ready when root Secret username is missing", func() {
		ns := "provider-missing-username"
		createProviderRootSecret(ctx, ns, "root", map[string][]byte{"password": []byte("rootpass")})
		provider := &surrealdbv1alpha1.SurrealDBProvider{ObjectMeta: metav1.ObjectMeta{Name: "provider-missing-username"}, Spec: surrealdbv1alpha1.SurrealDBProviderSpec{Endpoint: "ws://surrealdb:8000", RootCredentialRef: surrealdbv1alpha1.RootCredentialReference{Namespace: ns, Name: "root"}}}
		Expect(k8sClient.Create(ctx, provider)).To(Succeed())

		latest := reconcileProvider(ctx, provider.Name)

		ready := meta.FindStatusCondition(latest.Status.Conditions, operatorconditions.TypeReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(operatorconditions.ReasonProviderSecretInvalid))
	})

	It("marks provider not ready when root Secret password is missing", func() {
		ns := "provider-missing-password"
		createProviderRootSecret(ctx, ns, "root", map[string][]byte{"username": []byte("root")})
		provider := &surrealdbv1alpha1.SurrealDBProvider{ObjectMeta: metav1.ObjectMeta{Name: "provider-missing-password"}, Spec: surrealdbv1alpha1.SurrealDBProviderSpec{Endpoint: "ws://surrealdb:8000", RootCredentialRef: surrealdbv1alpha1.RootCredentialReference{Namespace: ns, Name: "root"}}}
		Expect(k8sClient.Create(ctx, provider)).To(Succeed())

		latest := reconcileProvider(ctx, provider.Name)

		ready := meta.FindStatusCondition(latest.Status.Conditions, operatorconditions.TypeReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(operatorconditions.ReasonProviderSecretInvalid))
	})

	It("rejects HTTP endpoints during provider validation", func() {
		provider := &surrealdbv1alpha1.SurrealDBProvider{Spec: surrealdbv1alpha1.SurrealDBProviderSpec{Endpoint: "http://surrealdb:8000"}}
		reconciler := &SurrealDBProviderReconciler{}

		err := reconciler.validateProvider(ctx, provider)

		Expect(err).To(MatchError(ContainSubstring("use ws:// or wss:// endpoints")))
	})
})

func reconcileProvider(ctx context.Context, name string) *surrealdbv1alpha1.SurrealDBProvider {
	reconciler := &SurrealDBProviderReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	Expect(err).NotTo(HaveOccurred())
	latest := &surrealdbv1alpha1.SurrealDBProvider{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, latest)).To(Succeed())
	return latest
}

func createProviderRootSecret(ctx context.Context, namespace, name string, data map[string][]byte) {
	createIgnoringAlreadyExists(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	Expect(k8sClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Data: data})).To(Succeed())
}
