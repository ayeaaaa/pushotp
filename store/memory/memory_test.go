package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"pushotp/store"
	"pushotp/store/storetest"
)

func TestMemoryBasics(t *testing.T) {
	s := New()
	defer s.Close()
	ctx := context.Background()
	now := time.Now()
	tk := store.Ticket{ID: "a", Receiver: "admin", CreatedAt: now, ExpiresAt: now.Add(time.Minute)}
	if err := s.SaveTicket(ctx, tk); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTicket(ctx, "a")
	if err != nil || got.ID != "a" {
		t.Fatalf("GetTicket = %+v, %v", got, err)
	}
	if _, err := s.GetTicket(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := s.MarkUsed(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetTicket(ctx, "a")
	if !got.Used {
		t.Fatal("ticket should be used")
	}
}

func TestMemoryContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		s := New()
		t.Cleanup(func() { s.Close() })
		return s
	})
}
