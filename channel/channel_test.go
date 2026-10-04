package channel

import (
	"context"
	"testing"
)

type stubChannel struct{}

func (s *stubChannel) Name() string { return "stub" }
func (s *stubChannel) Send(ctx context.Context, target Target, msg Message) error {
	return nil
}

func TestRegisterAndNew(t *testing.T) {
	Register("stub", func() Channel { return &stubChannel{} })
	if !Registered("stub") {
		t.Fatal("stub should be registered")
	}
	ch, err := New("stub")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if ch.Name() != "stub" {
		t.Fatalf("Name() = %q, want stub", ch.Name())
	}
}

func TestNewUnknownChannel(t *testing.T) {
	if _, err := New("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown channel")
	}
	if Registered("does-not-exist") {
		t.Fatal("unknown channel must not be registered")
	}
}
