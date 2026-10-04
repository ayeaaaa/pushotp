package storetest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ayeaaaa/pushotp/store"
)

func Run(t *testing.T, newStore func(t *testing.T) store.Store) {
	ctx := context.Background()

	t.Run("GetMissing", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.GetTicket(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("NilHashAndSalt", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		if err := s.SaveTicket(ctx, store.Ticket{ID: "nilhash", Receiver: "admin", CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
			t.Fatalf("SaveTicket with nil hash/salt: %v", err)
		}
		got, err := s.GetTicket(ctx, "nilhash")
		if err != nil {
			t.Fatal(err)
		}
		if len(got.CodeHash) != 0 || len(got.Salt) != 0 {
			t.Fatalf("hash/salt = %d/%d bytes, want empty", len(got.CodeHash), len(got.Salt))
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

	t.Run("IncrAttemptIfBelow", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		if err := s.SaveTicket(ctx, store.Ticket{ID: "t2", Receiver: "admin", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
			t.Fatal(err)
		}
		for want := 1; want <= 3; want++ {
			got, allowed, err := s.IncrAttemptIfBelow(ctx, "t2", 3)
			if err != nil || !allowed || got != want {
				t.Fatalf("IncrAttemptIfBelow = %d, %v, %v; want %d, true, nil", got, allowed, err, want)
			}
		}
		got, allowed, err := s.IncrAttemptIfBelow(ctx, "t2", 3)
		if err != nil || allowed || got != 3 {
			t.Fatalf("IncrAttemptIfBelow at max = %d, %v, %v; want 3, false, nil", got, allowed, err)
		}
		if _, _, err := s.IncrAttemptIfBelow(ctx, "missing", 3); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("MarkUsedIfUnused", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		if err := s.SaveTicket(ctx, store.Ticket{ID: "t3", Receiver: "admin", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
			t.Fatal(err)
		}
		ok, err := s.MarkUsedIfUnused(ctx, "t3")
		if err != nil || !ok {
			t.Fatalf("first MarkUsedIfUnused = %v, %v; want true, nil", ok, err)
		}
		ok, err = s.MarkUsedIfUnused(ctx, "t3")
		if err != nil || ok {
			t.Fatalf("second MarkUsedIfUnused = %v, %v; want false, nil", ok, err)
		}
		got, _ := s.GetTicket(ctx, "t3")
		if !got.Used {
			t.Fatal("ticket should be used")
		}
		if ok, err := s.MarkUsedIfUnused(ctx, "missing"); err != nil || ok {
			t.Fatalf("missing MarkUsedIfUnused = %v, %v; want false, nil", ok, err)
		}
	})

	t.Run("MarkUsedIfUnusedConcurrent", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		if err := s.SaveTicket(ctx, store.Ticket{ID: "cas", Receiver: "admin", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
			t.Fatal(err)
		}
		const n = 32
		var wg sync.WaitGroup
		var wins int32
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok, err := s.MarkUsedIfUnused(ctx, "cas")
				if err != nil {
					t.Errorf("MarkUsedIfUnused: %v", err)
					return
				}
				if ok {
					atomic.AddInt32(&wins, 1)
				}
			}()
		}
		wg.Wait()
		if wins != 1 {
			t.Fatalf("wins = %d, want 1", wins)
		}
	})

	t.Run("IncrAttemptIfBelowConcurrent", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		if err := s.SaveTicket(ctx, store.Ticket{ID: "attempts", Receiver: "admin", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
			t.Fatal(err)
		}
		const n = 32
		const max = 5
		var wg sync.WaitGroup
		var allowedCount int32
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, allowed, err := s.IncrAttemptIfBelow(ctx, "attempts", max); err != nil {
					t.Errorf("IncrAttemptIfBelow: %v", err)
				} else if allowed {
					atomic.AddInt32(&allowedCount, 1)
				}
			}()
		}
		wg.Wait()
		if allowedCount != max {
			t.Fatalf("allowed = %d, want %d", allowedCount, max)
		}
		got, err := s.GetTicket(ctx, "attempts")
		if err != nil {
			t.Fatal(err)
		}
		if got.Attempts != max {
			t.Fatalf("attempts = %d, want %d", got.Attempts, max)
		}
	})

	t.Run("FarFutureTimestamps", func(t *testing.T) {
		s := newStore(t)
		future := time.Date(2319, 1, 14, 0, 0, 0, 0, time.UTC)
		past := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := s.SaveTicket(ctx, store.Ticket{ID: "far", Receiver: "admin", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: past, ExpiresAt: future}); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetTicket(ctx, "far")
		if err != nil {
			t.Fatal(err)
		}
		if !got.ExpiresAt.Equal(future) {
			t.Fatalf("expires = %v, want %v", got.ExpiresAt, future)
		}
		if !got.CreatedAt.Equal(past) {
			t.Fatalf("created = %v, want %v", got.CreatedAt, past)
		}
	})

	t.Run("CountRecentAndLastSent", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		_ = s.SaveTicket(ctx, store.Ticket{ID: "old", Receiver: "admin", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: now.Add(-90 * time.Minute), ExpiresAt: now.Add(-80 * time.Minute)})
		_ = s.SaveTicket(ctx, store.Ticket{ID: "new", Receiver: "admin", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: now.Add(-30 * time.Minute), ExpiresAt: now.Add(30 * time.Minute)})
		_ = s.SaveTicket(ctx, store.Ticket{ID: "other", Receiver: "guest", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: now, ExpiresAt: now.Add(time.Minute)})
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

	t.Run("Ping", func(t *testing.T) {
		s := newStore(t)
		if err := s.Ping(ctx); err != nil {
			t.Fatalf("Ping: %v", err)
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		s := newStore(t)
		now := time.Now()
		_ = s.SaveTicket(ctx, store.Ticket{ID: "stale", Receiver: "admin", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)})
		_ = s.SaveTicket(ctx, store.Ticket{ID: "fresh", Receiver: "admin", CodeHash: []byte{1}, Salt: []byte{1}, CreatedAt: now, ExpiresAt: now.Add(time.Minute)})
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
