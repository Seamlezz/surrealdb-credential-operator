package rotation

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const AnnotationRotateAt = "surrealdb.seamlezz.com/rotate-at"

type Action string

const (
	ActionReuse    Action = "Reuse"
	ActionGenerate Action = "Generate"
	ActionRotate   Action = "Rotate"
)

type Decision struct {
	Action           Action
	Reason           string
	ManualTrigger    string
	NextRotationTime *metav1.Time
	LastRotationTime *metav1.Time
}

// Decide determines whether the reconciler should reuse, generate, or rotate the password.
func Decide(now time.Time, hasPassword bool, annotations map[string]string, lastManualTrigger string, lastRotationTime, nextRotationTime *metav1.Time, period *metav1.Duration) Decision {
	manual := ""
	if annotations != nil {
		manual = annotations[AnnotationRotateAt]
	}

	if !hasPassword {
		decision := Decision{Action: ActionGenerate, Reason: "PasswordMissing", ManualTrigger: manual}
		decision.LastRotationTime = timePtr(now)
		decision.NextRotationTime = computeNext(now, decision.LastRotationTime, nil, period)
		return decision
	}

	if manual != "" && manual != lastManualTrigger {
		decision := Decision{Action: ActionRotate, Reason: "ManualTriggerChanged", ManualTrigger: manual}
		decision.LastRotationTime = timePtr(now)
		decision.NextRotationTime = computeNext(now, decision.LastRotationTime, nil, period)
		return decision
	}

	if period != nil {
		next := computeNext(now, lastRotationTime, nextRotationTime, period)
		if next != nil && !now.Before(next.Time) {
			decision := Decision{Action: ActionRotate, Reason: "ScheduledRotationDue", ManualTrigger: lastManualTrigger}
			decision.LastRotationTime = timePtr(now)
			decision.NextRotationTime = computeNext(now, decision.LastRotationTime, nil, period)
			return decision
		}
		return Decision{Action: ActionReuse, Reason: "PasswordCurrent", ManualTrigger: lastManualTrigger, LastRotationTime: lastRotationTime, NextRotationTime: next}
	}

	return Decision{Action: ActionReuse, Reason: "PasswordCurrent", ManualTrigger: lastManualTrigger, LastRotationTime: lastRotationTime}
}

func computeNext(now time.Time, lastRotationTime, nextRotationTime *metav1.Time, period *metav1.Duration) *metav1.Time {
	if period == nil {
		return nil
	}

	base := now
	if lastRotationTime != nil {
		base = lastRotationTime.Time
	}
	candidate := timePtr(base.Add(period.Duration))

	if nextRotationTime == nil {
		return candidate
	}
	if nextRotationTime.Time.Before(candidate.Time) || nextRotationTime.Time.Equal(candidate.Time) {
		copy := *nextRotationTime
		return &copy
	}
	return candidate
}

func timePtr(t time.Time) *metav1.Time {
	mt := metav1.NewTime(t.UTC())
	return &mt
}
