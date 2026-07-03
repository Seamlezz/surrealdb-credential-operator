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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SurrealDBProviderSpec defines a platform-owned connection to a SurrealDB instance.
type SurrealDBProviderSpec struct {
	// Endpoint is the SurrealDB WS(S) endpoint used by the controller.
	// Examples: ws://surrealdb.surrealdb.svc.cluster.local:8000, wss://surrealdb.example.com/rpc.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^wss?://.+`
	Endpoint string `json:"endpoint"`

	// RootCredentialRef references the Kubernetes Secret containing SurrealDB admin credentials.
	RootCredentialRef RootCredentialReference `json:"rootCredentialRef"`

	// TLS configures optional TLS trust and client certificate settings for the provider endpoint.
	// +optional
	TLS *ProviderTLSConfig `json:"tls,omitempty"`
}

// RootCredentialReference references admin username/password keys in a Secret.
type RootCredentialReference struct {
	// Namespace is the Secret namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Namespace string `json:"namespace"`

	// Name is the Secret name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// UsernameKey is the Secret data key containing the admin username.
	// +kubebuilder:default=username
	// +kubebuilder:validation:MinLength=1
	UsernameKey string `json:"usernameKey,omitempty"`

	// PasswordKey is the Secret data key containing the admin password.
	// +kubebuilder:default=password
	// +kubebuilder:validation:MinLength=1
	PasswordKey string `json:"passwordKey,omitempty"`
}

// ProviderTLSConfig configures TLS options for a SurrealDBProvider.
type ProviderTLSConfig struct {
	// CASecretRef references a Secret key containing PEM-encoded CA certificates.
	// +optional
	CASecretRef *CASecretReference `json:"caSecretRef,omitempty"`

	// ClientCertificateRef references a Secret containing PEM-encoded client certificate and key material.
	// +optional
	ClientCertificateRef *ClientCertificateReference `json:"clientCertificateRef,omitempty"`

	// InsecureSkipVerify disables TLS certificate verification. This is intended only for local/dev use.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// SurrealDBProviderStatus defines observed state of SurrealDBProvider.
type SurrealDBProviderStatus struct {
	// ObservedGeneration is the latest metadata.generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the provider state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status

// SurrealDBProvider is the Schema for the surrealdbproviders API.
type SurrealDBProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines desired state of SurrealDBProvider.
	// +required
	Spec SurrealDBProviderSpec `json:"spec"`

	// status defines observed state of SurrealDBProvider.
	// +optional
	Status SurrealDBProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SurrealDBProviderList contains a list of SurrealDBProvider.
type SurrealDBProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SurrealDBProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SurrealDBProvider{}, &SurrealDBProviderList{})
}
