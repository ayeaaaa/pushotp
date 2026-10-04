package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ayeaaaa/pushotp/store"
	"github.com/ayeaaaa/pushotp/store/storetest"
)

func TestSQLiteContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		s, err := New(filepath.Join(t.TempDir(), "test.db"))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		t.Cleanup(func() { s.Close() })
		return s
	})
}

func TestDatabaseFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "perm.db")
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o, want 600", info.Mode().Perm())
	}
}

func TestURIDSNPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uri.db")
	s, err := New("file:" + path + "?mode=rwc")
	if err != nil {
		t.Fatalf("New URI DSN: %v", err)
	}
	defer s.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o, want 600", info.Mode().Perm())
	}
}

func TestMemoryDSN(t *testing.T) {
	s, err := New("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("New memory DSN: %v", err)
	}
	defer s.Close()
	if _, err := s.GetTicket(context.Background(), "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMigratesLegacyNanoTimestamps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Now().Add(-time.Hour)
	expires := time.Now().Add(time.Hour)
	if _, err := db.Exec(`CREATE TABLE tickets (
		id TEXT PRIMARY KEY, receiver TEXT NOT NULL, scene TEXT NOT NULL,
		code_hash BLOB NOT NULL, salt BLOB NOT NULL,
		expires_at INTEGER NOT NULL, created_at INTEGER NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 0, used INTEGER NOT NULL DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO tickets (id, receiver, scene, code_hash, salt, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "legacy", "admin", "login", []byte{1}, []byte{1}, expires.UnixNano(), created.UnixNano()); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()
	got, err := s.GetTicket(context.Background(), "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if diff := got.CreatedAt.Sub(created); diff > 2*time.Second || diff < -2*time.Second {
		t.Fatalf("created_at = %v, want ~%v", got.CreatedAt, created)
	}
	count, err := s.CountRecent(context.Background(), "admin", 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("CountRecent = %d, want 1", count)
	}
	last, err := s.LastSentAt(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if diff := last.Sub(created); diff > 2*time.Second || diff < -2*time.Second {
		t.Fatalf("LastSentAt = %v, want ~%v", last, created)
	}
}
