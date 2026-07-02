/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

// SurrealRole is a built-in SurrealDB system-user role.
// +kubebuilder:validation:Enum=VIEWER;EDITOR;OWNER
type SurrealRole string

const (
	// RoleViewer grants read-only permissions on the user's level and below.
	RoleViewer SurrealRole = "VIEWER"
	// RoleEditor grants edit permissions on the user's level and below, excluding IAM.
	RoleEditor SurrealRole = "EDITOR"
	// RoleOwner grants owner permissions on the user's level and below, including IAM.
	RoleOwner SurrealRole = "OWNER"
)

// UserLevel is the SurrealDB level where a generated system user is defined.
// +kubebuilder:validation:Enum=Namespace;Database
type UserLevel string

const (
	// UserLevelNamespace defines a user ON NAMESPACE.
	UserLevelNamespace UserLevel = "Namespace"
	// UserLevelDatabase defines a user ON DATABASE.
	UserLevelDatabase UserLevel = "Database"
)

// LocalProviderReference references a cluster-scoped SurrealDBProvider by name.
type LocalProviderReference struct {
	// Name is the name of the cluster-scoped SurrealDBProvider.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// LocalPolicyReference references a SurrealDBTenantPolicy in the same namespace as the credential.
type LocalPolicyReference struct {
	// Name is the same-namespace SurrealDBTenantPolicy name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// NamespacedSecretReference references a Kubernetes Secret.
type NamespacedSecretReference struct {
	// Namespace is the Secret namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Namespace string `json:"namespace"`

	// Name is the Secret name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// CASecretReference references a CA bundle key in a Kubernetes Secret.
type CASecretReference struct {
	// Namespace is the Secret namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Namespace string `json:"namespace"`

	// Name is the Secret name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Key is the Secret data key containing a PEM-encoded CA bundle.
	// +kubebuilder:default=ca.crt
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key,omitempty"`
}

// ClientCertificateReference references client certificate material in a Kubernetes Secret.
type ClientCertificateReference struct {
	// Namespace is the Secret namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Namespace string `json:"namespace"`

	// Name is the Secret name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// CertKey is the Secret data key containing a PEM-encoded client certificate.
	// +kubebuilder:default=tls.crt
	// +kubebuilder:validation:MinLength=1
	CertKey string `json:"certKey,omitempty"`

	// KeyKey is the Secret data key containing a PEM-encoded client private key.
	// +kubebuilder:default=tls.key
	// +kubebuilder:validation:MinLength=1
	KeyKey string `json:"keyKey,omitempty"`
}
