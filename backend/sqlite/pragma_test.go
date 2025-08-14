package sqlite

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_SqliteBackend_PragmaSettings(t *testing.T) {
	t.Run("In-memory database has memory journal mode", func(t *testing.T) {
		backend := NewInMemoryBackend()
		defer backend.Close()

		// Check that journal_mode is set to memory for in-memory databases
		// (WAL mode is not supported for in-memory databases)
		var journalMode string
		err := backend.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
		require.NoError(t, err)
		require.Equal(t, "memory", journalMode, "journal_mode should be 'memory' for in-memory databases")
	})

	t.Run("File backend has WAL mode", func(t *testing.T) {
		// Test with a temporary file-based backend
		backend := NewSqliteBackend("test_pragma.db")
		defer backend.Close()

		// Check that journal_mode is set to WAL for file databases
		var journalMode string
		err := backend.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
		require.NoError(t, err)
		require.Equal(t, "wal", journalMode, "journal_mode should be set to WAL for file backend")
	})
}