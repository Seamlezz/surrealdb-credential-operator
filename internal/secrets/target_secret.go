package secrets

import (
	"fmt"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelName       = "app.kubernetes.io/name"
	LabelManagedBy  = "app.kubernetes.io/managed-by"
	LabelCredential = "surrealdb.seamlezz.com/credential"
	ManagedByValue  = "surrealdb-credential-operator"
)

// ConnectionData is rendered into the generated target Secret.
type ConnectionData struct {
	URL       string
	Namespace string
	Database  string
	Username  string
	Password  string
	Level     string
}

// TargetSecretName returns the output Secret name for a credential.
func TargetSecretName(credential *api.SurrealDBCredential) string {
	return credential.Spec.TargetSecret.Name
}

// BuildTargetSecret builds the desired generated Secret for a SurrealDBCredential.
func BuildTargetSecret(credential *api.SurrealDBCredential, data ConnectionData) *corev1.Secret {
	controller := true
	blockOwnerDeletion := true
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      credential.Spec.TargetSecret.Name,
			Namespace: credential.Namespace,
			Labels: map[string]string{
				LabelName:       ManagedByValue,
				LabelManagedBy:  ManagedByValue,
				LabelCredential: credential.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         api.GroupVersion.String(),
					Kind:               "SurrealDBCredential",
					Name:               credential.Name,
					UID:                credential.UID,
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"url":          []byte(data.URL),
			"namespace":    []byte(data.Namespace),
			"database":     []byte(data.Database),
			"username":     []byte(data.Username),
			"password":     []byte(data.Password),
			"level":        []byte(data.Level),
			"SURREAL_URL":  []byte(data.URL),
			"SURREAL_NS":   []byte(data.Namespace),
			"SURREAL_DB":   []byte(data.Database),
			"SURREAL_USER": []byte(data.Username),
			"SURREAL_PASS": []byte(data.Password),
		},
	}
}

// IsOwnedByCredential returns true when secret is controller-owned by credential.
func IsOwnedByCredential(secret *corev1.Secret, credential *api.SurrealDBCredential) bool {
	for _, owner := range secret.OwnerReferences {
		if owner.APIVersion == api.GroupVersion.String() && owner.Kind == "SurrealDBCredential" && owner.Name == credential.Name && owner.UID == credential.UID {
			return true
		}
	}
	return false
}

// EnsureCanManage returns an error if an existing Secret is not owned by credential.
func EnsureCanManage(secret *corev1.Secret, credential *api.SurrealDBCredential) error {
	if secret == nil {
		return nil
	}
	if IsOwnedByCredential(secret, credential) {
		return nil
	}
	return fmt.Errorf("target Secret %s/%s already exists and is not owned by SurrealDBCredential %s/%s", secret.Namespace, secret.Name, credential.Namespace, credential.Name)
}

// ExistingPassword returns password data from an existing target Secret.
func ExistingPassword(secret *corev1.Secret) (string, bool) {
	if secret == nil || secret.Data == nil {
		return "", false
	}
	password, ok := secret.Data["password"]
	if !ok || len(password) == 0 {
		return "", false
	}
	return string(password), true
}
