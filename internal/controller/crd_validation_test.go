package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("CRD validation", func() {
	ctx := context.Background()

	It("rejects a SurrealDBCredential without required spec fields", func() {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("surrealdb.seamlezz.com/v1alpha1")
		obj.SetKind("SurrealDBCredential")
		obj.SetName("invalid")
		obj.SetNamespace("default")
		err := k8sClient.Create(ctx, obj)
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected invalid error, got %v", err)
	})

	It("rejects invalid role enum values", func() {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "surrealdb.seamlezz.com/v1alpha1",
			"kind":       "SurrealDBCredential",
			"metadata": map[string]any{
				"name":      "invalid-role",
				"namespace": "default",
			},
			"spec": map[string]any{
				"policyRef": map[string]any{"name": "smoke"},
				"level":     "Database",
				"database":  "smoke",
				"roles":     []any{"ADMIN"},
				"targetSecret": map[string]any{
					"name": "creds",
				},
			},
		}}
		obj.SetCreationTimestamp(metav1.Time{})
		err := k8sClient.Create(ctx, obj)
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected invalid error, got %v", err)
	})

	It("rejects SurrealDBProvider HTTP endpoints", func() {
		obj := providerWithEndpoint("invalid-http", "http://surrealdb:8000")
		err := k8sClient.Create(ctx, obj)
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected invalid error, got %v", err)
	})

	It("rejects SurrealDBProvider HTTPS endpoints", func() {
		obj := providerWithEndpoint("invalid-https", "https://surrealdb:8000")
		err := k8sClient.Create(ctx, obj)
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected invalid error, got %v", err)
	})

	It("accepts SurrealDBProvider WS endpoints", func() {
		obj := providerWithEndpoint("valid-ws", "ws://surrealdb:8000")
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
	})

	It("accepts SurrealDBProvider WSS endpoints", func() {
		obj := providerWithEndpoint("valid-wss", "wss://surrealdb.example.com/rpc")
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
	})
})

func providerWithEndpoint(name, endpoint string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "surrealdb.seamlezz.com/v1alpha1",
		"kind":       "SurrealDBProvider",
		"metadata": map[string]any{
			"name": name,
		},
		"spec": map[string]any{
			"endpoint": endpoint,
			"rootCredentialRef": map[string]any{
				"namespace": "platform-secrets",
				"name":      "surrealdb-root",
			},
		},
	}}
	obj.SetCreationTimestamp(metav1.Time{})
	return obj
}
