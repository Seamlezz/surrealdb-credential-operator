package surreal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoadTLSConfigLoadsCA(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	ca := testCACertPEM(t)
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: "surrealdb"},
		Data:       map[string][]byte{"ca.crt": ca},
	}).Build()

	cfg, err := LoadTLSConfig(context.Background(), reader, &api.ProviderTLSConfig{CASecretRef: &api.CASecretReference{Namespace: "surrealdb", Name: "ca", Key: "ca.crt"}})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.RootCAs == nil {
		t.Fatal("expected RootCAs to be set")
	}
}

func TestLoadTLSConfigReportsMissingCAKey(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: "surrealdb"},
		Data:       map[string][]byte{},
	}).Build()
	_, err := LoadTLSConfig(context.Background(), reader, &api.ProviderTLSConfig{CASecretRef: &api.CASecretReference{Namespace: "surrealdb", Name: "ca", Key: "ca.crt"}})
	if err == nil {
		t.Fatal("expected missing CA key error")
	}
}

func TestLoadTLSConfigInsecureSkipVerify(t *testing.T) {
	cfg, err := LoadTLSConfig(context.Background(), fake.NewClientBuilder().Build(), &api.ProviderTLSConfig{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || !cfg.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify to be true")
	}
}

func testCACertPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
