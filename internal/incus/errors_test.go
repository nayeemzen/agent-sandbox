package incus

import (
	"errors"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	if IsNotFound(nil) {
		t.Fatalf("expected false for nil")
	}
	if !IsNotFound(errors.New("not found")) {
		t.Fatalf("expected true for \"not found\"")
	}
	if !IsNotFound(errors.New("Instance not found")) {
		t.Fatalf("expected true for mixed case")
	}
	if IsNotFound(errors.New("permission denied")) {
		t.Fatalf("expected false for unrelated errors")
	}
}

func TestIsAlreadyExists(t *testing.T) {
	t.Parallel()

	if IsAlreadyExists(nil) {
		t.Fatalf("expected false for nil")
	}
	if !IsAlreadyExists(errors.New("already exists")) {
		t.Fatalf("expected true for already exists")
	}
	if !IsAlreadyExists(errors.New(`This "instances" entry already exists`)) {
		t.Fatalf("expected true for Incus already-exists error")
	}
	if IsAlreadyExists(errors.New("not found")) {
		t.Fatalf("expected false for unrelated errors")
	}
}
