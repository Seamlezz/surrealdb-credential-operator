package secrets

import (
	"strings"
	"testing"
)

func TestGeneratePasswordLength(t *testing.T) {
	got, err := GeneratePassword(48)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 48 {
		t.Fatalf("len = %d, want 48", len(got))
	}
}

func TestGeneratePasswordAlphabet(t *testing.T) {
	got, err := GeneratePassword(128)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if !strings.ContainsRune(passwordAlphabet, r) {
			t.Fatalf("password contains unexpected rune %q", r)
		}
	}
}

func TestGeneratePasswordRejectsInvalidLength(t *testing.T) {
	if _, err := GeneratePassword(0); err == nil {
		t.Fatal("expected invalid length error")
	}
}

func TestGeneratePasswordProducesDifferentValues(t *testing.T) {
	first, err := GeneratePassword(48)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GeneratePassword(48)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("expected two generated passwords to differ")
	}
}
