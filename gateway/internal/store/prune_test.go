package store

import (
	"context"
	"testing"
	"time"
)

func TestPruneAuditRemovesOnlyOldEntries(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	if _, err := m.CreateKey(ctx, "k", 0); err != nil {
		t.Fatalf("create key: %v", err)
	}
	for range 3 {
		if err := m.RecordAudit(ctx, Usage{KeyName: "k", Model: "m", Status: 200}); err != nil {
			t.Fatalf("record audit: %v", err)
		}
	}

	// Nothing is older than an hour ago, so nothing may be removed.
	removed, err := m.PruneAudit(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 0 {
		t.Fatalf("pruned %d recent entries; want 0", removed)
	}
	entries, err := m.AuditList(ctx, "", 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("have %d entries after a no-op prune; want 3", len(entries))
	}

	// A cutoff in the future matches everything.
	removed, err = m.PruneAudit(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 3 {
		t.Fatalf("pruned %d; want 3", removed)
	}
	entries, err = m.AuditList(ctx, "", 100)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("have %d entries after pruning everything; want 0", len(entries))
	}
}

// Pruning an empty log is a no-op, not an error — the retention loop runs on a
// timer and must tolerate having nothing to do.
func TestPruneAuditOnEmptyLog(t *testing.T) {
	removed, err := NewMemory().PruneAudit(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 0 {
		t.Fatalf("pruned %d from an empty log; want 0", removed)
	}
}
