package readstate

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStoreMarkReadAndUnread(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ref := "owner/repo#1"
	if err := store.MarkRead(ctx, ref, []string{"e1", "e2"}); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	got, err := store.ListRead(ctx, ref, []string{"e1", "e2", "e3"})
	if err != nil {
		t.Fatalf("list read: %v", err)
	}
	if !got["e1"] || !got["e2"] {
		t.Fatalf("expected e1 and e2 to be read, got=%v", got)
	}
	if got["e3"] {
		t.Fatalf("did not expect e3 to be read, got=%v", got)
	}

	if err := store.MarkUnread(ctx, ref, []string{"e2"}); err != nil {
		t.Fatalf("mark unread: %v", err)
	}
	got, err = store.ListRead(ctx, ref, []string{"e1", "e2"})
	if err != nil {
		t.Fatalf("list read after unread: %v", err)
	}
	if !got["e1"] {
		t.Fatalf("expected e1 to stay read, got=%v", got)
	}
	if got["e2"] {
		t.Fatalf("expected e2 to be unread, got=%v", got)
	}
}

func TestStorePersistsAcrossReopen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	ref := "owner/repo#2"

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.MarkRead(ctx, ref, []string{"e1"}); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	got, err := reopened.ListRead(ctx, ref, []string{"e1", "e2"})
	if err != nil {
		t.Fatalf("list read: %v", err)
	}
	if !got["e1"] {
		t.Fatalf("expected e1 to remain read across reopen, got=%v", got)
	}
	if got["e2"] {
		t.Fatalf("did not expect e2 to be read, got=%v", got)
	}
}
