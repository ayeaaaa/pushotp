package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"pushotp/store"
)

const schema = `
CREATE TABLE IF NOT EXISTS tickets (
	id TEXT PRIMARY KEY,
	receiver TEXT NOT NULL,
	scene TEXT NOT NULL,
	code_hash BLOB NOT NULL,
	salt BLOB NOT NULL,
	expires_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	attempts INTEGER NOT NULL DEFAULT 0,
	used INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_tickets_receiver_created ON tickets(receiver, created_at);
`

const legacyNanoThreshold = 1_000_000_000_000_000

type Store struct {
	db   *sql.DB
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: schema: %w", err)
	}
	if _, err := db.Exec(`UPDATE tickets SET created_at = created_at / 1000000, expires_at = expires_at / 1000000
		WHERE ABS(created_at) >= ? OR ABS(expires_at) >= ?`, legacyNanoThreshold, legacyNanoThreshold); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: migrate legacy timestamps: %w", err)
	}
	if path != "" && path != ":memory:" {
		if err := os.Chmod(path, 0o600); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite: chmod: %w", err)
		}
	}
	s := &Store{db: db, stop: make(chan struct{}), done: make(chan struct{})}
	go s.janitor()
	return s, nil
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) SaveTicket(ctx context.Context, t store.Ticket) error {
	codeHash := t.CodeHash
	if codeHash == nil {
		codeHash = []byte{}
	}
	salt := t.Salt
	if salt == nil {
		salt = []byte{}
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tickets (id, receiver, scene, code_hash, salt, expires_at, created_at, attempts, used)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Receiver, t.Scene, codeHash, salt,
		t.ExpiresAt.UnixMilli(), t.CreatedAt.UnixMilli(), t.Attempts, boolToInt(t.Used))
	if err != nil {
		return fmt.Errorf("sqlite: save ticket: %w", err)
	}
	return nil
}

func (s *Store) GetTicket(ctx context.Context, id string) (*store.Ticket, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, receiver, scene, code_hash, salt, expires_at, created_at, attempts, used
		 FROM tickets WHERE id = ?`, id)
	var t store.Ticket
	var expiresAt, createdAt int64
	var used int
	if err := row.Scan(&t.ID, &t.Receiver, &t.Scene, &t.CodeHash, &t.Salt, &expiresAt, &createdAt, &t.Attempts, &used); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("sqlite: get ticket: %w", err)
	}
	t.ExpiresAt = time.UnixMilli(expiresAt)
	t.CreatedAt = time.UnixMilli(createdAt)
	t.Used = used != 0
	return &t, nil
}

func (s *Store) DeleteTicket(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM tickets WHERE id = ?`, id); err != nil {
		return fmt.Errorf("sqlite: delete ticket: %w", err)
	}
	return nil
}

func (s *Store) IncrAttemptIfBelow(ctx context.Context, id string, max int) (int, bool, error) {
	var attempts int
	err := s.db.QueryRowContext(ctx,
		`UPDATE tickets SET attempts = attempts + 1 WHERE id = ? AND attempts < ? RETURNING attempts`,
		id, max).Scan(&attempts)
	if err == nil {
		return attempts, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, fmt.Errorf("sqlite: incr attempt: %w", err)
	}
	var current int
	if err := s.db.QueryRowContext(ctx, `SELECT attempts FROM tickets WHERE id = ?`, id).Scan(&current); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, store.ErrNotFound
		}
		return 0, false, fmt.Errorf("sqlite: incr attempt: %w", err)
	}
	return current, false, nil
}

func (s *Store) MarkUsedIfUnused(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE tickets SET used = 1 WHERE id = ? AND used = 0`, id)
	if err != nil {
		return false, fmt.Errorf("sqlite: mark used: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("sqlite: mark used rows: %w", err)
	}
	return n == 1, nil
}

func (s *Store) CountRecent(ctx context.Context, receiver string, window time.Duration) (int, error) {
	var count int
	cutoff := time.Now().Add(-window).UnixMilli()
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tickets WHERE receiver = ? AND created_at >= ?`, receiver, cutoff).Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlite: count recent: %w", err)
	}
	return count, nil
}

func (s *Store) LastSentAt(ctx context.Context, receiver string) (time.Time, error) {
	var maxCreated sql.NullInt64
	if err := s.db.QueryRowContext(ctx,
		`SELECT MAX(created_at) FROM tickets WHERE receiver = ?`, receiver).Scan(&maxCreated); err != nil {
		return time.Time{}, fmt.Errorf("sqlite: last sent: %w", err)
	}
	if !maxCreated.Valid {
		return time.Time{}, nil
	}
	return time.UnixMilli(maxCreated.Int64), nil
}

func (s *Store) Cleanup(ctx context.Context, now time.Time) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM tickets WHERE expires_at < ? AND created_at < ?`,
		now.UnixMilli(), now.Add(-time.Hour).UnixMilli()); err != nil {
		return fmt.Errorf("sqlite: cleanup: %w", err)
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) Close() error {
	s.once.Do(func() { close(s.stop) })
	<-s.done
	return s.db.Close()
}
