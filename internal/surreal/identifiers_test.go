package surreal

import "testing"

func TestEscapeIdentSimple(t *testing.T) {
	got, err := EscapeIdent("smoke_db")
	if err != nil {
		t.Fatal(err)
	}
	if got != "smoke_db" {
		t.Fatalf("got %q, want smoke_db", got)
	}
}

func TestEscapeIdentReserved(t *testing.T) {
	got, err := EscapeIdent("select")
	if err != nil {
		t.Fatal(err)
	}
	if got != "`select`" {
		t.Fatalf("got %q, want reserved identifier escaped", got)
	}
}

func TestEscapeIdentSpecialCharacters(t *testing.T) {
	got, err := EscapeIdent("smoke-db.prod")
	if err != nil {
		t.Fatal(err)
	}
	if got != "`smoke-db.prod`" {
		t.Fatalf("got %q", got)
	}
}

func TestEscapeIdentEscapesBacktick(t *testing.T) {
	got, err := EscapeIdent("evil`name")
	if err != nil {
		t.Fatal(err)
	}
	if got != "`evil\\`name`" {
		t.Fatalf("got %q", got)
	}
}

func TestEscapeIdentRejectsControlCharacter(t *testing.T) {
	if _, err := EscapeIdent("bad\nname"); err == nil {
		t.Fatal("expected error for control character")
	}
}

func TestEscapeIdentRejectsEmpty(t *testing.T) {
	if _, err := EscapeIdent(""); err == nil {
		t.Fatal("expected error for empty identifier")
	}
}
