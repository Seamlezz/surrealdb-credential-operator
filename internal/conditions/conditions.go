package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TypeReady          = "Ready"
	TypePolicyAccepted = "PolicyAccepted"
	TypeUserDefined    = "UserDefined"
	TypeSecretReady    = "SecretReady"
)

const (
	ReasonReconciled             = "Reconciled"
	ReasonAvailable              = "Available"
	ReasonPolicyAccepted         = "Allowed"
	ReasonPolicyDenied           = "PolicyDenied"
	ReasonProviderNotFound       = "ProviderNotFound"
	ReasonProviderSecretNotFound = "ProviderSecretNotFound"
	ReasonProviderSecretInvalid  = "ProviderSecretInvalid"
	ReasonPolicyNotFound         = "PolicyNotFound"
	ReasonSecretConflict         = "SecretConflict"
	ReasonSecretDeleteFailed     = "SecretDeleteFailed"
	ReasonTargetNotFound         = "TargetNotFound"
	ReasonSurrealDBUnavailable   = "SurrealDBUnavailable"
	ReasonDefineUserFailed       = "DefineUserFailed"
	ReasonRemoveUserFailed       = "RemoveUserFailed"
	ReasonRotationFailed         = "RotationFailed"
	ReasonForceCleanupRequested  = "ForceCleanupRequested"
	ReasonSecretWritten          = "SecretWritten"
	ReasonDefineUserSucceeded    = "DefineUserSucceeded"
	ReasonDeleting               = "Deleting"
)

// Set updates a condition in-place using the standard metav1.Condition shape.
func Set(conditions *[]metav1.Condition, generation int64, conditionType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: generation,
		Reason:             reason,
		Message:            message,
	})
}

// Ready marks the Ready condition.
func Ready(conditions *[]metav1.Condition, generation int64, reason, message string) {
	Set(conditions, generation, TypeReady, metav1.ConditionTrue, reason, message)
}

// NotReady marks the Ready condition false.
func NotReady(conditions *[]metav1.Condition, generation int64, reason, message string) {
	Set(conditions, generation, TypeReady, metav1.ConditionFalse, reason, message)
}

// Unknown marks the Ready condition unknown.
func Unknown(conditions *[]metav1.Condition, generation int64, reason, message string) {
	Set(conditions, generation, TypeReady, metav1.ConditionUnknown, reason, message)
}
