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

// SurrealDBCredentialSpec defines a request for one generated SurrealDB system user and Secret.
// +kubebuilder:validation:XValidation:rule="(self.level != 'Database' || (has(self.database) && self.database.size() > 0)) && (self.level != 'Namespace' || !has(self.database) || self.database.size() == 0)",message="database is required when level is Database and must be omitted or empty when level is Namespace"
type SurrealDBCredentialSpec struct {
	// PolicyRef references the same-namespace SurrealDBTenantPolicy authorizing this request.
	PolicyRef LocalPolicyReference `json:"policyRef"`

	// Level is the SurrealDB level where the user is defined.
	Level UserLevel `json:"level"`

	// Database is the SurrealDB database for database-level users.
	// Required when level is Database. Omit for namespace-level users.
	// +optional
	// +kubebuilder:validation:MaxLength=128
	Database string `json:"database,omitempty"`

	// Roles is the non-empty set of SurrealDB roles to grant.
	// +listType=set
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=3
	Roles []SurrealRole `json:"roles"`

	// TargetSecret describes the generated Secret. It is always written to the same namespace as this CR.
	TargetSecret TargetSecretSpec `json:"targetSecret"`

	// Rotation configures optional scheduled password rotation.
	// +optional
	Rotation *RotationSpec `json:"rotation,omitempty"`
}

// TargetSecretSpec identifies the same-namespace output Secret.
type TargetSecretSpec struct {
	// Name is the name of the generated Kubernetes Secret in the same namespace as the credential CR.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`
}

// RotationSpec configures password rotation.
type RotationSpec struct {
	// Period is the optional scheduled rotation period.
	// +optional
	Period *metav1.Duration `json:"period,omitempty"`
}

// SurrealDBCredentialStatus defines observed state of SurrealDBCredential.
type SurrealDBCredentialStatus struct {
	// ObservedGeneration is the latest metadata.generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ProviderRef is the provider resolved via the tenant policy.
	// +optional
	ProviderRef *LocalProviderReference `json:"providerRef,omitempty"`

	// SurrealNamespace is the resolved SurrealDB namespace.
	// +optional
	SurrealNamespace string `json:"surrealNamespace,omitempty"`

	// Database is the resolved SurrealDB database for database-level credentials.
	// +optional
	Database string `json:"database,omitempty"`

	// Username is the deterministic SurrealDB username managed by this credential.
	// +optional
	Username string `json:"username,omitempty"`

	// LastRotationTime is the last time the managed password was generated or rotated.
	// +optional
	LastRotationTime *metav1.Time `json:"lastRotationTime,omitempty"`

	// NextRotationTime is the next scheduled rotation time when spec.rotation.period is set.
	// +optional
	NextRotationTime *metav1.Time `json:"nextRotationTime,omitempty"`

	// LastManualRotationTrigger records the last processed rotate-at annotation value.
	// +optional
	LastManualRotationTrigger string `json:"lastManualRotationTrigger,omitempty"`

	// Conditions represent the latest available observations of the credential state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SurrealDBCredential is the Schema for the surrealdbcredentials API.
type SurrealDBCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines desired state of SurrealDBCredential.
	// +required
	Spec SurrealDBCredentialSpec `json:"spec"`

	// status defines observed state of SurrealDBCredential.
	// +optional
	Status SurrealDBCredentialStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SurrealDBCredentialList contains a list of SurrealDBCredential.
type SurrealDBCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SurrealDBCredential `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SurrealDBCredential{}, &SurrealDBCredentialList{})
}
