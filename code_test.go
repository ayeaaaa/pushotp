package pushotp

import (
	"strings"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	for _, length := range []int{4, 6, 8} {
		code, err := generateCode(length)
		if err != nil {
			t.Fatalf("generateCode(%d): %v", length, err)
		}
		if len(code) != length {
			t.Fatalf("len(code) = %d, want %d", len(code), length)
		}
		if strings.Trim(code, "0123456789") != "" {
			t.Fatalf("code %q contains non-digit characters", code)
		}
	}
}

func TestGenerateCodeInvalidLength(t *testing.T) {
	for _, length := range []int{0, 3, 9} {
		if _, err := generateCode(length); err == nil {
			t.Fatalf("generateCode(%d): expected error", length)
		}
	}
}

func TestHashCodeMatchesOnlySameInput(t *testing.T) {
	salt := []byte("0123456789abcdef")
	a := hashCode(salt, "123456")
	b := hashCode(salt, "123456")
	c := hashCode(salt, "654321")
	if !equalHash(a, b) {
		t.Fatal("same code and salt must match")
	}
	if equalHash(a, c) {
		t.Fatal("different code must not match")
	}
	if equalHash(a, hashCode([]byte("other-salt-1234"), "123456")) {
		t.Fatal("different salt must not match")
	}
}

func TestNewID(t *testing.T) {
	a, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("ids must be unique")
	}
	if len(a) != 32 {
		t.Fatalf("len(id) = %d, want 32", len(a))
	}
}
