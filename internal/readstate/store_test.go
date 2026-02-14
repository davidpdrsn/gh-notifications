package readstate

import (
	"context"
	"path/filepath"
	"testing"
	"time"
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

func TestStoreMarkParentReadAndUnread(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ref1 := "owner/repo#1"
	ref2 := "owner/repo#2"
	if err := store.MarkParentRead(ctx, ref1); err != nil {
		t.Fatalf("mark parent read: %v", err)
	}

	got, err := store.ListParentRead(ctx, []string{ref1, ref2})
	if err != nil {
		t.Fatalf("list parent read: %v", err)
	}
	if !got[ref1] {
		t.Fatalf("expected %q to be parent-read, got=%v", ref1, got)
	}
	if got[ref2] {
		t.Fatalf("did not expect %q to be parent-read, got=%v", ref2, got)
	}

	if err := store.MarkParentUnread(ctx, ref1); err != nil {
		t.Fatalf("mark parent unread: %v", err)
	}
	got, err = store.ListParentRead(ctx, []string{ref1, ref2})
	if err != nil {
		t.Fatalf("list parent read after unread: %v", err)
	}
	if got[ref1] {
		t.Fatalf("expected %q to be parent-unread, got=%v", ref1, got)
	}
}

func TestStoreParentReadPersistsAcrossReopen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	ref := "owner/repo#3"

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.MarkParentRead(ctx, ref); err != nil {
		t.Fatalf("mark parent read: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	got, err := reopened.ListParentRead(ctx, []string{ref})
	if err != nil {
		t.Fatalf("list parent read: %v", err)
	}
	if !got[ref] {
		t.Fatalf("expected %q to remain parent-read across reopen, got=%v", ref, got)
	}
}

func TestStoreMarkThreadArchivedAndUnmark(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	threadID := "123"
	archivedUpdatedAt := time.Now().UTC().Add(-time.Minute).Round(0)
	if err := store.MarkThreadArchived(ctx, threadID, archivedUpdatedAt); err != nil {
		t.Fatalf("mark thread archived: %v", err)
	}

	got, err := store.ListArchivedThreads(ctx)
	if err != nil {
		t.Fatalf("list archived threads: %v", err)
	}
	if ts, ok := got[threadID]; !ok {
		t.Fatalf("expected archived thread %q, got=%v", threadID, got)
	} else if !ts.Equal(archivedUpdatedAt) {
		t.Fatalf("expected archived_updated_at=%v, got=%v", archivedUpdatedAt, ts)
	}

	if err := store.UnmarkThreadArchived(ctx, threadID); err != nil {
		t.Fatalf("unmark thread archived: %v", err)
	}
	got, err = store.ListArchivedThreads(ctx)
	if err != nil {
		t.Fatalf("list archived threads after unmark: %v", err)
	}
	if _, ok := got[threadID]; ok {
		t.Fatalf("expected archived thread to be removed, got=%v", got)
	}
}

func TestStoreArchivedThreadPersistsAcrossReopen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	threadID := "456"
	archivedUpdatedAt := time.Now().UTC().Round(0)

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.MarkThreadArchived(ctx, threadID, archivedUpdatedAt); err != nil {
		t.Fatalf("mark thread archived: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	got, err := reopened.ListArchivedThreads(ctx)
	if err != nil {
		t.Fatalf("list archived threads: %v", err)
	}
	if ts, ok := got[threadID]; !ok {
		t.Fatalf("expected archived thread %q, got=%v", threadID, got)
	} else if !ts.Equal(archivedUpdatedAt) {
		t.Fatalf("expected archived_updated_at=%v, got=%v", archivedUpdatedAt, ts)
	}
}
