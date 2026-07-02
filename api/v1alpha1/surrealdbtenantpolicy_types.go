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

// SurrealDBTenantPolicySpec defines allowed SurrealDB credential requests for one Kubernetes namespace.
type SurrealDBTenantPolicySpec struct {
	// ProviderRef references the cluster-scoped SurrealDBProvider this policy allows.
	ProviderRef LocalProviderReference `json:"providerRef"`

	// SurrealNamespace is the SurrealDB namespace app credentials may target.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	SurrealNamespace string `json:"surrealNamespace"`

	// DatabaseUsers controls database-level user requests.
	// +optional
	DatabaseUsers DatabaseUserPolicy `json:"databaseUsers,omitempty"`

	// NamespaceUsers controls namespace-level user requests.
	// +optional
	NamespaceUsers NamespaceUserPolicy `json:"namespaceUsers,omitempty"`
}

// DatabaseUserPolicy controls allowed database-level users.
type DatabaseUserPolicy struct {
	// AllowedDatabases is the exact list of SurrealDB databases this namespace may target.
	// +optional
	// +listType=set
	// +kubebuilder:validation:MaxItems=256
	AllowedDatabases []string `json:"allowedDatabases,omitempty"`

	// AllowedRoles is the set of SurrealDB roles database-level users may request.
	// +optional
	// +listType=set
	// +kubebuilder:validation:MaxItems=3
	AllowedRoles []SurrealRole `json:"allowedRoles,omitempty"`
}

// NamespaceUserPolicy controls allowed namespace-level users.
type NamespaceUserPolicy struct {
	// Allowed enables namespace-level user requests.
	// +optional
	Allowed bool `json:"allowed,omitempty"`

	// AllowedRoles is the set of SurrealDB roles namespace-level users may request.
	// +optional
	// +listType=set
	// +kubebuilder:validation:MaxItems=3
	AllowedRoles []SurrealRole `json:"allowedRoles,omitempty"`
}

// SurrealDBTenantPolicyStatus defines observed state of SurrealDBTenantPolicy.
type SurrealDBTenantPolicyStatus struct {
	// ObservedGeneration is the latest metadata.generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ProviderRef is the provider resolved by the controller.
	// +optional
	ProviderRef *LocalProviderReference `json:"providerRef,omitempty"`

	// Conditions represent the latest available observations of the tenant policy state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SurrealDBTenantPolicy is the Schema for the surrealdbtenantpolicies API.
type SurrealDBTenantPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines desired state of SurrealDBTenantPolicy.
	// +required
	Spec SurrealDBTenantPolicySpec `json:"spec"`

	// status defines observed state of SurrealDBTenantPolicy.
	// +optional
	Status SurrealDBTenantPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SurrealDBTenantPolicyList contains a list of SurrealDBTenantPolicy.
type SurrealDBTenantPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SurrealDBTenantPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SurrealDBTenantPolicy{}, &SurrealDBTenantPolicyList{})
}
