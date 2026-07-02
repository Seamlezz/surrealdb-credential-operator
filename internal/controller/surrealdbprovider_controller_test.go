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

var _ = Describe("SurrealDBProvider Controller", func() {
	ctx := context.Background()

	It("marks provider ready when root Secret keys exist", func() {
		ns := "provider-ready"
		createIgnoringAlreadyExists(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		Expect(k8sClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: ns}, Data: map[string][]byte{"username": []byte("root"), "password": []byte("rootpass")}})).To(Succeed())
		provider := &surrealdbv1alpha1.SurrealDBProvider{ObjectMeta: metav1.ObjectMeta{Name: "provider-ready"}, Spec: surrealdbv1alpha1.SurrealDBProviderSpec{Endpoint: "http://surrealdb:8000", RootCredentialRef: surrealdbv1alpha1.RootCredentialReference{Namespace: ns, Name: "root"}}}
		Expect(k8sClient.Create(ctx, provider)).To(Succeed())

		reconciler := &SurrealDBProviderReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: provider.Name}})
		Expect(err).NotTo(HaveOccurred())

		latest := &surrealdbv1alpha1.SurrealDBProvider{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: provider.Name}, latest)).To(Succeed())
		Expect(latest.Status.Conditions).NotTo(BeEmpty())
	})
})
