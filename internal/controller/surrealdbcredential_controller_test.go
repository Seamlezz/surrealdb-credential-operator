/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package controller

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	surrealdbv1alpha1 "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	operatorconditions "github.com/Seamlezz/surrealdb-credential-operator/internal/conditions"
	"github.com/Seamlezz/surrealdb-credential-operator/internal/surreal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type fakeAdmin struct {
	defined   []string
	removed   []string
	removeErr error
}

func (f *fakeAdmin) DefineUser(ctx context.Context, target surreal.UserTarget, username, password string, roles []surrealdbv1alpha1.SurrealRole) error {
	f.defined = append(f.defined, username+":"+password)
	return nil
}
func (f *fakeAdmin) RemoveUser(ctx context.Context, target surreal.UserTarget, username string) error {
	f.removed = append(f.removed, username)
	return f.removeErr
}
func (f *fakeAdmin) Ping(ctx context.Context) error  { return nil }
func (f *fakeAdmin) Close(ctx context.Context) error { return nil }

type failingSecretDeleteClient struct {
	client.Client
	err error
}

func (c failingSecretDeleteClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, ok := obj.(*corev1.Secret); ok && obj.GetName() == "surrealdb-smoke-credentials" {
		return c.err
	}
	return c.Client.Delete(ctx, obj, opts...)
}

var _ = Describe("SurrealDBCredential Controller", func() {
	ctx := context.Background()

	It("creates a SurrealDB user and target Secret", func() {
		ns := "cred-create"
		admin := &fakeAdmin{}
		createCredentialFixture(ctx, ns)
		reconciler := testCredentialReconciler(admin)
		req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "smoke-editor"}}

		_, err := reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred()) // adds finalizer
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "surrealdb-smoke-credentials"}, secret)).To(Succeed())
		Expect(string(secret.Data["username"])).NotTo(BeEmpty())
		Expect(string(secret.Data["password"])).NotTo(BeEmpty())
		Expect(string(secret.Data["SURREAL_USER"])).To(Equal(string(secret.Data["username"])))
		Expect(admin.defined).To(HaveLen(1))
	})

	It("marks policy denied without touching SurrealDB", func() {
		ns := "cred-denied"
		admin := &fakeAdmin{}
		createCredentialFixture(ctx, ns)
		cred := &surrealdbv1alpha1.SurrealDBCredential{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "smoke-editor"}, cred)).To(Succeed())
		cred.Spec.Roles = []surrealdbv1alpha1.SurrealRole{surrealdbv1alpha1.RoleOwner}
		Expect(k8sClient.Update(ctx, cred)).To(Succeed())

		reconciler := testCredentialReconciler(admin)
		req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "smoke-editor"}}
		_, err := reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(admin.defined).To(BeEmpty())
	})

	It("rejects an existing unowned target Secret", func() {
		ns := "cred-conflict"
		admin := &fakeAdmin{}
		createCredentialFixture(ctx, ns)
		Expect(k8sClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "surrealdb-smoke-credentials", Namespace: ns}})).To(Succeed())

		reconciler := testCredentialReconciler(admin)
		req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "smoke-editor"}}
		_, err := reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(admin.defined).To(BeEmpty())
	})

	It("removes the SurrealDB user and owned Secret on delete", func() {
		ns := "cred-delete"
		admin := &fakeAdmin{}
		createCredentialFixture(ctx, ns)
		reconciler := testCredentialReconciler(admin)
		req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "smoke-editor"}}
		_, err := reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		cred := &surrealdbv1alpha1.SurrealDBCredential{}
		Expect(k8sClient.Get(ctx, req.NamespacedName, cred)).To(Succeed())
		Expect(k8sClient.Delete(ctx, cred)).To(Succeed())
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(admin.removed).NotTo(BeEmpty())

		secret := &corev1.Secret{}
		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: "surrealdb-smoke-credentials"}, secret)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("keeps the finalizer and records status when normal target Secret deletion fails", func() {
		ns := "cred-secret-delete-fails"
		admin := &fakeAdmin{}
		createCredentialFixture(ctx, ns)
		deleteErr := errors.New("delete failed")
		reconciler := testCredentialReconcilerWithClient(admin, failingSecretDeleteClient{Client: k8sClient, err: deleteErr})
		req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "smoke-editor"}}
		_, err := reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		cred := &surrealdbv1alpha1.SurrealDBCredential{}
		Expect(k8sClient.Get(ctx, req.NamespacedName, cred)).To(Succeed())
		Expect(k8sClient.Delete(ctx, cred)).To(Succeed())
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).To(MatchError(deleteErr))

		updated := &surrealdbv1alpha1.SurrealDBCredential{}
		Expect(k8sClient.Get(ctx, req.NamespacedName, updated)).To(Succeed())
		Expect(updated.Finalizers).To(ContainElement(credentialFinalizer))
		ready := meta.FindStatusCondition(updated.Status.Conditions, operatorconditions.TypeReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(operatorconditions.ReasonSecretDeleteFailed))
		secretReady := meta.FindStatusCondition(updated.Status.Conditions, operatorconditions.TypeSecretReady)
		Expect(secretReady).NotTo(BeNil())
		Expect(secretReady.Status).To(Equal(metav1.ConditionFalse))
		Expect(secretReady.Reason).To(Equal(operatorconditions.ReasonSecretDeleteFailed))
	})

	It("force cleanup skips SurrealDB removal and removes the finalizer despite target Secret deletion failure", func() {
		ns := "cred-force-secret-delete-fails"
		admin := &fakeAdmin{}
		createCredentialFixture(ctx, ns)
		deleteErr := errors.New("delete failed")
		reconciler := testCredentialReconcilerWithClient(admin, failingSecretDeleteClient{Client: k8sClient, err: deleteErr})
		req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "smoke-editor"}}
		_, err := reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		cred := &surrealdbv1alpha1.SurrealDBCredential{}
		Expect(k8sClient.Get(ctx, req.NamespacedName, cred)).To(Succeed())
		Expect(k8sClient.Delete(ctx, cred)).To(Succeed())

		terminating := &surrealdbv1alpha1.SurrealDBCredential{}
		Expect(k8sClient.Get(ctx, req.NamespacedName, terminating)).To(Succeed())
		terminating.Annotations = map[string]string{annotationForceCleanup: "true"}
		Expect(k8sClient.Update(ctx, terminating)).To(Succeed())

		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(admin.removed).To(BeEmpty())
		Eventually(func(g Gomega) {
			deleted := &surrealdbv1alpha1.SurrealDBCredential{}
			err := k8sClient.Get(ctx, req.NamespacedName, deleted)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}).Should(Succeed())
	})

	It("keeps the finalizer and records status when SurrealDB user removal fails", func() {
		ns := "cred-remove-user-fails"
		removeErr := errors.New("remove failed")
		admin := &fakeAdmin{removeErr: removeErr}
		createCredentialFixture(ctx, ns)
		reconciler := testCredentialReconciler(admin)
		req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: "smoke-editor"}}
		_, err := reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).NotTo(HaveOccurred())

		cred := &surrealdbv1alpha1.SurrealDBCredential{}
		Expect(k8sClient.Get(ctx, req.NamespacedName, cred)).To(Succeed())
		Expect(k8sClient.Delete(ctx, cred)).To(Succeed())
		_, err = reconciler.Reconcile(ctx, req)
		Expect(err).To(MatchError(removeErr))

		updated := &surrealdbv1alpha1.SurrealDBCredential{}
		Expect(k8sClient.Get(ctx, req.NamespacedName, updated)).To(Succeed())
		Expect(updated.Finalizers).To(ContainElement(credentialFinalizer))
		ready := meta.FindStatusCondition(updated.Status.Conditions, operatorconditions.TypeReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		Expect(ready.Reason).To(Equal(operatorconditions.ReasonRemoveUserFailed))
	})
})

func testCredentialReconciler(admin *fakeAdmin) *SurrealDBCredentialReconciler {
	return testCredentialReconcilerWithClient(admin, k8sClient)
}

func testCredentialReconcilerWithClient(admin *fakeAdmin, c client.Client) *SurrealDBCredentialReconciler {
	return &SurrealDBCredentialReconciler{
		Client: c,
		Scheme: k8sClient.Scheme(),
		Clock:  func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) },
		AdminFactory: func(ctx context.Context, endpoint, username, password string, tlsConfig *tls.Config) (surreal.Admin, error) {
			return admin, nil
		},
	}
}

func createCredentialFixture(ctx context.Context, namespace string) {
	createIgnoringAlreadyExists(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	Expect(k8sClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: namespace}, Data: map[string][]byte{"username": []byte("root"), "password": []byte("rootpass")}})).To(Succeed())
	Expect(k8sClient.Create(ctx, &surrealdbv1alpha1.SurrealDBProvider{ObjectMeta: metav1.ObjectMeta{Name: "default-" + namespace}, Spec: surrealdbv1alpha1.SurrealDBProviderSpec{Endpoint: "ws://surrealdb:8000", RootCredentialRef: surrealdbv1alpha1.RootCredentialReference{Namespace: namespace, Name: "root"}}})).To(Succeed())
	Expect(k8sClient.Create(ctx, &surrealdbv1alpha1.SurrealDBTenantPolicy{ObjectMeta: metav1.ObjectMeta{Name: "smoke", Namespace: namespace}, Spec: surrealdbv1alpha1.SurrealDBTenantPolicySpec{ProviderRef: surrealdbv1alpha1.LocalProviderReference{Name: "default-" + namespace}, SurrealNamespace: "smoke", DatabaseUsers: surrealdbv1alpha1.DatabaseUserPolicy{AllowedDatabases: []string{"smoke"}, AllowedRoles: []surrealdbv1alpha1.SurrealRole{surrealdbv1alpha1.RoleViewer, surrealdbv1alpha1.RoleEditor}}}})).To(Succeed())
	Expect(k8sClient.Create(ctx, &surrealdbv1alpha1.SurrealDBCredential{ObjectMeta: metav1.ObjectMeta{Name: "smoke-editor", Namespace: namespace}, Spec: surrealdbv1alpha1.SurrealDBCredentialSpec{PolicyRef: surrealdbv1alpha1.LocalPolicyReference{Name: "smoke"}, Level: surrealdbv1alpha1.UserLevelDatabase, Database: "smoke", Roles: []surrealdbv1alpha1.SurrealRole{surrealdbv1alpha1.RoleEditor}, TargetSecret: surrealdbv1alpha1.TargetSecretSpec{Name: "surrealdb-smoke-credentials"}}})).To(Succeed())
}

func createIgnoringAlreadyExists(ctx context.Context, obj client.Object) {
	err := k8sClient.Create(ctx, obj)
	if apierrors.IsAlreadyExists(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred())
}
