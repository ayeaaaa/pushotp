package storetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"pushotp/store"
)

func Run(t *testing.T, newStore func(t *testing.T) store.Store) {
	ctx := context.Background()

	t.Run("GetMissing", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.GetTicket(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("SaveGetDelete", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		tk := store.Ticket{
			ID: "t1", Receiver: "admin", Scene: "login",
			CodeHash: []byte{1}, Salt: []byte{2},
			CreatedAt: now, ExpiresAt: now.Add(time.Minute),
		}
		if err := s.SaveTicket(ctx, tk); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetTicket(ctx, "t1")
		if err != nil {
			t.Fatal(err)
		}
		if got.Receiver != "admin" || got.Scene != "login" || got.Used || got.Attempts != 0 {
			t.Fatalf("unexpected ticket: %+v", got)
		}
		if err := s.DeleteTicket(ctx, "t1"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.GetTicket(ctx, "t1"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("IncrAttempt", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		if err := s.SaveTicket(ctx, store.Ticket{ID: "t2", Receiver: "admin", CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
			t.Fatal(err)
		}
		for want := 1; want <= 3; want++ {
			got, err := s.IncrAttempt(ctx, "t2")
			if err != nil || got != want {
				t.Fatalf("IncrAttempt = %d, %v; want %d", got, err, want)
			}
		}
		if _, err := s.IncrAttempt(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("MarkUsed", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		if err := s.SaveTicket(ctx, store.Ticket{ID: "t3", Receiver: "admin", CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
			t.Fatal(err)
		}
		if err := s.MarkUsed(ctx, "t3"); err != nil {
			t.Fatal(err)
		}
		got, _ := s.GetTicket(ctx, "t3")
		if !got.Used {
			t.Fatal("ticket should be used")
		}
		if err := s.MarkUsed(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("CountRecentAndLastSent", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		_ = s.SaveTicket(ctx, store.Ticket{ID: "old", Receiver: "admin", CreatedAt: now.Add(-90 * time.Minute), ExpiresAt: now.Add(-80 * time.Minute)})
		_ = s.SaveTicket(ctx, store.Ticket{ID: "new", Receiver: "admin", CreatedAt: now.Add(-30 * time.Minute), ExpiresAt: now.Add(30 * time.Minute)})
		_ = s.SaveTicket(ctx, store.Ticket{ID: "other", Receiver: "guest", CreatedAt: now, ExpiresAt: now.Add(time.Minute)})
		count, err := s.CountRecent(ctx, "admin", time.Hour)
		if err != nil || count != 1 {
			t.Fatalf("CountRecent = %d, %v; want 1", count, err)
		}
		last, err := s.LastSentAt(ctx, "admin")
		if err != nil {
			t.Fatal(err)
		}
		if last.Before(now.Add(-31*time.Minute)) || last.After(now.Add(-29*time.Minute)) {
			t.Fatalf("LastSentAt = %v", last)
		}
		if zero, _ := s.LastSentAt(ctx, "nobody"); !zero.IsZero() {
			t.Fatalf("LastSentAt unknown = %v, want zero", zero)
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		_ = s.SaveTicket(ctx, store.Ticket{ID: "stale", Receiver: "admin", CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)})
		_ = s.SaveTicket(ctx, store.Ticket{ID: "fresh", Receiver: "admin", CreatedAt: now, ExpiresAt: now.Add(time.Minute)})
		if err := s.Cleanup(ctx, now); err != nil {
			t.Fatal(err)
		}
		if _, err := s.GetTicket(ctx, "stale"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("stale err = %v, want ErrNotFound", err)
		}
		if _, err := s.GetTicket(ctx, "fresh"); err != nil {
			t.Fatalf("fresh should remain: %v", err)
		}
	})
}
