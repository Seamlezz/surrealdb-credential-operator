package surreal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LoadTLSConfig loads provider TLS material from referenced Kubernetes Secrets.
func LoadTLSConfig(ctx context.Context, reader client.Reader, cfg *api.ProviderTLSConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, nil
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify} //nolint:gosec // explicitly user-controlled provider option

	if cfg.CASecretRef != nil {
		secret := &corev1.Secret{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: cfg.CASecretRef.Namespace, Name: cfg.CASecretRef.Name}, secret); err != nil {
			return nil, fmt.Errorf("get CA secret %s/%s: %w", cfg.CASecretRef.Namespace, cfg.CASecretRef.Name, err)
		}
		key := cfg.CASecretRef.Key
		if key == "" {
			key = "ca.crt"
		}
		pem, ok := secret.Data[key]
		if !ok || len(pem) == 0 {
			return nil, fmt.Errorf("CA secret %s/%s missing key %q", secret.Namespace, secret.Name, key)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("CA secret %s/%s key %q does not contain PEM certificates", secret.Namespace, secret.Name, key)
		}
		tlsConfig.RootCAs = pool
	}

	if cfg.ClientCertificateRef != nil {
		secret := &corev1.Secret{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: cfg.ClientCertificateRef.Namespace, Name: cfg.ClientCertificateRef.Name}, secret); err != nil {
			return nil, fmt.Errorf("get client certificate secret %s/%s: %w", cfg.ClientCertificateRef.Namespace, cfg.ClientCertificateRef.Name, err)
		}
		certKey := cfg.ClientCertificateRef.CertKey
		if certKey == "" {
			certKey = "tls.crt"
		}
		keyKey := cfg.ClientCertificateRef.KeyKey
		if keyKey == "" {
			keyKey = "tls.key"
		}
		certPEM, ok := secret.Data[certKey]
		if !ok || len(certPEM) == 0 {
			return nil, fmt.Errorf("client certificate secret %s/%s missing key %q", secret.Namespace, secret.Name, certKey)
		}
		keyPEM, ok := secret.Data[keyKey]
		if !ok || len(keyPEM) == 0 {
			return nil, fmt.Errorf("client certificate secret %s/%s missing key %q", secret.Namespace, secret.Name, keyKey)
		}
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("parse client certificate secret %s/%s: %w", secret.Namespace, secret.Name, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
