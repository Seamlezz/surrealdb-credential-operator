package rotation

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDecideGeneratesWhenPasswordMissing(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	period := metav1.Duration{Duration: 24 * time.Hour}
	got := Decide(now, false, nil, "", nil, nil, &period)
	if got.Action != ActionGenerate {
		t.Fatalf("action = %s, want %s", got.Action, ActionGenerate)
	}
	if got.LastRotationTime == nil || !got.LastRotationTime.Time.Equal(now) {
		t.Fatalf("last rotation = %v, want %v", got.LastRotationTime, now)
	}
	if got.NextRotationTime == nil || !got.NextRotationTime.Time.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("next rotation = %v", got.NextRotationTime)
	}
}

func TestDecideRotatesOnManualTriggerChange(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	got := Decide(now, true, map[string]string{AnnotationRotateAt: now.Format(time.RFC3339)}, "", nil, nil, nil)
	if got.Action != ActionRotate {
		t.Fatalf("action = %s, want %s", got.Action, ActionRotate)
	}
	if got.ManualTrigger == "" {
		t.Fatal("expected manual trigger recorded")
	}
}

func TestDecideReusesWhenManualTriggerAlreadyProcessed(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	trigger := now.Format(time.RFC3339)
	got := Decide(now, true, map[string]string{AnnotationRotateAt: trigger}, trigger, nil, nil, nil)
	if got.Action != ActionReuse {
		t.Fatalf("action = %s, want %s", got.Action, ActionReuse)
	}
}

func TestDecideRotatesWhenScheduleDue(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	next := metav1.NewTime(now.Add(-time.Minute))
	period := metav1.Duration{Duration: time.Hour}
	got := Decide(now, true, nil, "", nil, &next, &period)
	if got.Action != ActionRotate {
		t.Fatalf("action = %s, want %s", got.Action, ActionRotate)
	}
	if got.NextRotationTime == nil || !got.NextRotationTime.Time.Equal(now.Add(time.Hour)) {
		t.Fatalf("next rotation = %v", got.NextRotationTime)
	}
}

func TestDecideReusesBeforeScheduleDue(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	next := metav1.NewTime(now.Add(time.Minute))
	period := metav1.Duration{Duration: time.Hour}
	got := Decide(now, true, nil, "", nil, &next, &period)
	if got.Action != ActionReuse {
		t.Fatalf("action = %s, want %s", got.Action, ActionReuse)
	}
}
