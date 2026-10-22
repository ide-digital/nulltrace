package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenMigrateSeedless(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := Open(ctx, filepath.Join(dir, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM brokers`).Scan(&n); err != nil {
		t.Fatal(err)
	}
}

func TestClaimActionOnlyOnce(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	now := nowRFC3339()
	for _, q := range []string{
		`INSERT INTO identities VALUES ('i1', x'00', x'00', NULL, NULL, '` + now + `', '` + now + `')`,
		`INSERT INTO brokers(id, name, domain, category, mechanism, jurisdiction_coverage) VALUES ('b1', 'B', 'b.test', 'PEOPLE_SEARCH', 'EMAIL_LEGAL', 'US')`,
		`INSERT INTO exposed_records VALUES ('r1', 'i1', 'b1', NULL, x'00', 0.5, 'DISCOVERED', 'MEDIUM', '` + now + `', '` + now + `')`,
		`INSERT INTO erasure_actions(id, exposed_record_id, broker_id, strategy, current_state, created_at, updated_at) VALUES ('a1', 'r1', 'b1', 'SMTP_DEMAND', 'QUEUED', '` + now + `', '` + now + `')`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	s := New(db)
	first, err := s.ClaimAction(ctx, "a1", "QUEUED", "SENDING")
	if err != nil || !first {
		t.Fatalf("first claim: %v %v", first, err)
	}
	second, err := s.ClaimAction(ctx, "a1", "QUEUED", "SENDING")
	if err != nil || second {
		t.Fatalf("second claim should lose: %v %v", second, err)
	}

	if err := s.UpdateActionState(ctx, "a1", "FAILED", 1, "dial tcp: timeout", nil); err != nil {
		t.Fatal(err)
	}
	back, err := s.RequeueFailed(ctx, "r1", "SMTP_DEMAND")
	if err != nil || !back {
		t.Fatalf("requeue: %v %v", back, err)
	}
	again, err := s.ClaimAction(ctx, "a1", "QUEUED", "SENDING")
	if err != nil || !again {
		t.Fatalf("requeued action should be claimable: %v %v", again, err)
	}
}

func TestStaleVerifiedComparesTimesNotText(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	// RFC3339Nano drops trailing zeros: "...00Z" sorts after "...00.5Z" as text.
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 500_000_000, time.UTC)
	verified := "2026-01-01T00:00:00Z"
	for _, q := range []string{
		`INSERT INTO identities VALUES ('i1', x'00', x'00', NULL, NULL, '` + verified + `', '` + verified + `')`,
		`INSERT INTO brokers(id, name, domain, category, mechanism, jurisdiction_coverage) VALUES ('b1', 'B', 'b.test', 'PEOPLE_SEARCH', 'EMAIL_LEGAL', 'US')`,
		`INSERT INTO exposed_records VALUES ('r1', 'i1', 'b1', NULL, x'00', 0.5, 'VERIFIED_REMOVED', 'MEDIUM', '` + verified + `', '` + verified + `')`,
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	stale, err := New(db).StaleVerified(ctx, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected the record to be stale, got %d", len(stale))
	}
}
