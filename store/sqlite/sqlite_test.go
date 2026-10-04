package sqlite

import (
	"path/filepath"
	"testing"

	"pushotp/store"
	"pushotp/store/storetest"
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
