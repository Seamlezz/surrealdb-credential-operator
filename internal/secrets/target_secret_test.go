package secrets

import (
	"testing"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func testCredential() *api.SurrealDBCredential {
	return &api.SurrealDBCredential{
		TypeMeta:   metav1.TypeMeta{APIVersion: api.GroupVersion.String(), Kind: "SurrealDBCredential"},
		ObjectMeta: metav1.ObjectMeta{Name: "smoke-editor", Namespace: "smoke", UID: types.UID("uid-1")},
		Spec: api.SurrealDBCredentialSpec{
			TargetSecret: api.TargetSecretSpec{Name: "surrealdb-smoke-credentials"},
		},
	}
}

func TestBuildTargetSecretIncludesCanonicalKeysAndAliases(t *testing.T) {
	secret := BuildTargetSecret(testCredential(), ConnectionData{
		URL: "ws://surrealdb", Namespace: "smoke", Database: "smoke", Username: "user", Password: "pass", Level: "database",
	})
	want := map[string]string{
		"url": "ws://surrealdb", "namespace": "smoke", "database": "smoke", "username": "user", "password": "pass", "level": "database",
		"SURREAL_URL": "ws://surrealdb", "SURREAL_NS": "smoke", "SURREAL_DB": "smoke", "SURREAL_USER": "user", "SURREAL_PASS": "pass",
	}
	for key, value := range want {
		if string(secret.Data[key]) != value {
			t.Fatalf("secret key %s = %q, want %q", key, string(secret.Data[key]), value)
		}
	}
}

func TestBuildTargetSecretSetsOwnerReference(t *testing.T) {
	cred := testCredential()
	secret := BuildTargetSecret(cred, ConnectionData{})
	if !IsOwnedByCredential(secret, cred) {
		t.Fatalf("expected secret to be owned by credential: %#v", secret.OwnerReferences)
	}
}

func TestEnsureCanManageAllowsNilAndOwnedSecret(t *testing.T) {
	cred := testCredential()
	if err := EnsureCanManage(nil, cred); err != nil {
		t.Fatalf("nil secret should be manageable: %v", err)
	}
	secret := BuildTargetSecret(cred, ConnectionData{})
	if err := EnsureCanManage(secret, cred); err != nil {
		t.Fatalf("owned secret should be manageable: %v", err)
	}
}

func TestEnsureCanManageRejectsUnownedSecret(t *testing.T) {
	cred := testCredential()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "surrealdb-smoke-credentials", Namespace: "smoke"}}
	if err := EnsureCanManage(secret, cred); err == nil {
		t.Fatal("expected conflict for unowned secret")
	}
}

func TestExistingPassword(t *testing.T) {
	secret := &corev1.Secret{Data: map[string][]byte{"password": []byte("secret")}}
	got, ok := ExistingPassword(secret)
	if !ok || got != "secret" {
		t.Fatalf("ExistingPassword = %q, %v", got, ok)
	}
	_, ok = ExistingPassword(&corev1.Secret{})
	if ok {
		t.Fatal("expected missing password")
	}
}
