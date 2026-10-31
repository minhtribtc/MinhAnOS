package jobcore

import (
	"context"
	"errors"
	"testing"
)

// Enqueue joins the caller's transaction: a rollback leaves no job, a commit leaves exactly one queued job, dedupe
// holds inside the transaction, and an invalid type is refused.
func TestEnqueueInTransaction(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	count := func() int {
		var n int
		if e := db.QueryRow(ctx, "SELECT count(*) FROM platform_jobs WHERE type='test.tx'").Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}

	// Rolled back: the job never exists.
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Enqueue(ctx, tx, Spec{Type: "test.tx"}); e != nil {
		t.Fatal(e)
	}
	if e = tx.Rollback(ctx); e != nil {
		t.Fatal(e)
	}
	if n := count(); n != 0 {
		t.Fatal("rolled back job visible", n)
	}

	// Committed: exactly one queued job.
	tx, e = db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	j, e := Enqueue(ctx, tx, Spec{Type: "test.tx"})
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	var state string
	if e = db.QueryRow(ctx, "SELECT state FROM platform_jobs WHERE id=$1", j.ID).Scan(&state); e != nil || state != "queued" {
		t.Fatal("committed job", state, e)
	}
	if n := count(); n != 1 {
		t.Fatal("committed count", n)
	}

	// Dedupe inside one transaction: the second Enqueue hands back the job queued by the first.
	tx, e = db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	a, e := Enqueue(ctx, tx, Spec{Type: "test.tx", DedupeKey: "tx"})
	if e != nil {
		t.Fatal(e)
	}
	b, e := Enqueue(ctx, tx, Spec{Type: "test.tx", DedupeKey: "tx"})
	if e != nil {
		t.Fatal(e)
	}
	if b.ID != a.ID {
		t.Fatal("dedupe in transaction", a.ID, b.ID)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if n := count(); n != 2 {
		t.Fatal("dedupe queued twice", n)
	}

	// An invalid type is refused before touching the database.
	if _, e = Enqueue(ctx, db, Spec{Type: "invalid"}); !errors.Is(e, ErrInvalid) {
		t.Fatal("invalid type", e)
	}
	if n := count(); n != 2 {
		t.Fatal("invalid type queued", n)
	}
}
