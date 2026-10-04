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
	IncrAttempt(ctx context.Context, id string) (int, error)
	MarkUsed(ctx context.Context, id string) error
	CountRecent(ctx context.Context, receiver string, window time.Duration) (int, error)
	LastSentAt(ctx context.Context, receiver string) (time.Time, error)
	Cleanup(ctx context.Context, now time.Time) error
	Close() error
}
