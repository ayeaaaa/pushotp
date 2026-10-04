package store

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("store: ticket not found")

type Ticket struct {
	ID        string
	Receiver  string
	Scene     string
	CodeHash  []byte
	Salt      []byte
	ExpiresAt time.Time
	CreatedAt time.Time
	Attempts  int
	Used      bool
}

type Store interface {
	SaveTicket(ctx context.Context, t Ticket) error
	GetTicket(ctx context.Context, id string) (*Ticket, error)
	DeleteTicket(ctx context.Context, id string) error
	IncrAttemptIfBelow(ctx context.Context, id string, max int) (int, bool, error)
	MarkUsedIfUnused(ctx context.Context, id string) (bool, error)
	CountRecent(ctx context.Context, receiver string, window time.Duration) (int, error)
	LastSentAt(ctx context.Context, receiver string) (time.Time, error)
	Cleanup(ctx context.Context, now time.Time) error
	Ping(ctx context.Context) error
	Close() error
}
