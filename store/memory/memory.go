package memory

import (
	"context"
	"sync"
	"time"

	"pushotp/store"
)

type Store struct {
	mu      sync.Mutex
	tickets map[string]store.Ticket
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
}

func New() *Store {
	s := &Store{
		tickets: make(map[string]store.Ticket),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go s.janitor()
	return s
}

func (s *Store) janitor() {
	defer close(s.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case now := <-ticker.C:
			_ = s.Cleanup(context.Background(), now)
		}
	}
}

func cloneTicket(t store.Ticket) store.Ticket {
	t.CodeHash = append([]byte(nil), t.CodeHash...)
	t.Salt = append([]byte(nil), t.Salt...)
	return t
}

func (s *Store) SaveTicket(ctx context.Context, t store.Ticket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.CodeHash == nil {
		t.CodeHash = []byte{}
	}
	if t.Salt == nil {
		t.Salt = []byte{}
	}
	s.tickets[t.ID] = cloneTicket(t)
	return nil
}

func (s *Store) GetTicket(ctx context.Context, id string) (*store.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	c := cloneTicket(t)
	return &c, nil
}

func (s *Store) DeleteTicket(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tickets, id)
	return nil
}

func (s *Store) IncrAttempt(ctx context.Context, id string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[id]
	if !ok {
		return 0, store.ErrNotFound
	}
	t.Attempts++
	s.tickets[id] = t
	return t.Attempts, nil
}

func (s *Store) MarkUsed(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[id]
	if !ok {
		return store.ErrNotFound
	}
	t.Used = true
	s.tickets[id] = t
	return nil
}

func (s *Store) CountRecent(ctx context.Context, receiver string, window time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-window)
	count := 0
	for _, t := range s.tickets {
		if t.Receiver == receiver && !t.CreatedAt.Before(cutoff) {
			count++
		}
	}
	return count, nil
}

func (s *Store) LastSentAt(ctx context.Context, receiver string) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var last time.Time
	for _, t := range s.tickets {
		if t.Receiver == receiver && t.CreatedAt.After(last) {
			last = t.CreatedAt
		}
	}
	return last, nil
}

func (s *Store) Cleanup(ctx context.Context, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.tickets {
		if now.After(t.ExpiresAt) && now.Sub(t.CreatedAt) > time.Hour {
			delete(s.tickets, id)
		}
	}
	return nil
}

func (s *Store) Close() error {
	s.once.Do(func() { close(s.stop) })
	<-s.done
	return nil
}
