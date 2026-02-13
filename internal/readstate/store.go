package readstate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store struct {
	path string
	mu   sync.Mutex
}

func Open(path string) (*Store, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, fmt.Errorf("read-state path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return nil, fmt.Errorf("create read-state directory: %w", err)
	}

	store := &Store{path: cleanPath}
	if err := store.migrate(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS event_read_state (
  ref      TEXT NOT NULL,
  event_id TEXT NOT NULL,
  read_at  TEXT NOT NULL,
  source   TEXT NOT NULL,
  PRIMARY KEY (ref, event_id)
);
CREATE INDEX IF NOT EXISTS idx_event_read_state_read_at
  ON event_read_state(read_at);`
	_, err := s.execSQL(ctx, schema)
	if err != nil {
		return fmt.Errorf("migrate read-state schema: %w", err)
	}
	return nil
}

func (s *Store) MarkRead(ctx context.Context, ref string, eventIDs []string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("ref is empty")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var b strings.Builder
	b.WriteString("BEGIN;")
	for _, id := range eventIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		b.WriteString("INSERT INTO event_read_state(ref, event_id, read_at, source) VALUES(")
		b.WriteString(sqlQuote(ref))
		b.WriteString(",")
		b.WriteString(sqlQuote(id))
		b.WriteString(",")
		b.WriteString(sqlQuote(now))
		b.WriteString(",")
		b.WriteString(sqlQuote("manual"))
		b.WriteString(") ON CONFLICT(ref, event_id) DO UPDATE SET read_at=excluded.read_at, source=excluded.source;")
	}
	b.WriteString("COMMIT;")

	if _, err := s.execSQL(ctx, b.String()); err != nil {
		return fmt.Errorf("mark events as read: %w", err)
	}
	return nil
}

func (s *Store) MarkUnread(ctx context.Context, ref string, eventIDs []string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("ref is empty")
	}

	var b strings.Builder
	b.WriteString("BEGIN;")
	for _, id := range eventIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		b.WriteString("DELETE FROM event_read_state WHERE ref=")
		b.WriteString(sqlQuote(ref))
		b.WriteString(" AND event_id=")
		b.WriteString(sqlQuote(id))
		b.WriteString(";")
	}
	b.WriteString("COMMIT;")

	if _, err := s.execSQL(ctx, b.String()); err != nil {
		return fmt.Errorf("mark events as unread: %w", err)
	}
	return nil
}

func (s *Store) ListRead(ctx context.Context, ref string, eventIDs []string) (map[string]bool, error) {
	out := make(map[string]bool)
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return out, fmt.Errorf("ref is empty")
	}

	ids := make([]string, 0, len(eventIDs))
	for _, id := range eventIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return out, nil
	}

	var b strings.Builder
	b.WriteString("SELECT event_id FROM event_read_state WHERE ref=")
	b.WriteString(sqlQuote(ref))
	b.WriteString(" AND event_id IN (")
	for i, id := range ids {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(sqlQuote(id))
	}
	b.WriteString(");")

	stdout, err := s.execSQL(ctx, b.String())
	if err != nil {
		return out, fmt.Errorf("list read events: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out[line] = true
	}
	return out, nil
}

func (s *Store) execSQL(ctx context.Context, sql string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("read-state store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := exec.CommandContext(ctx, "sqlite3", "-batch", "-noheader", s.path, sql)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("sqlite3 failed: %s", msg)
	}
	return string(out), nil
}

func sqlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
