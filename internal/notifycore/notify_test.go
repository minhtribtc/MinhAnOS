package notifycore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	alice = "11111111-1111-4111-8111-111111111111"
	bob   = "22222222-2222-4222-8222-222222222222"
)

func fixture(t *testing.T) Store {
	t.Helper()
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated notification database not configured")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("notify_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
		t.Fatal(e)
	}
	config, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	config.ConnConfig.RuntimeParams["search_path"] = name
	db, e := pgxpool.NewWithConfig(ctx, config)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(); admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); admin.Close() })
	// Stand-in for the identity-owned outbox (identity SQL 007), only the columns the worker reads.
	if _, e = db.Exec(ctx, `CREATE TABLE identity_notification_outbox(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),recipient_id uuid NOT NULL,kind text NOT NULL,severity text NOT NULL,mandatory boolean NOT NULL DEFAULT false,details jsonb NOT NULL DEFAULT '{}',request_id text NOT NULL,correlation_id text,created_at timestamptz NOT NULL DEFAULT now(),delivered_at timestamptz)`); e != nil {
		t.Fatal(e)
	}
	if e = ApplySchema(ctx, db); e != nil {
		t.Fatal(e)
	}
	return Store{DB: db}
}

func notice(t *testing.T, s Store, recipient, kind string, age time.Duration) {
	t.Helper()
	if _, e := s.DB.Exec(context.Background(), "INSERT INTO identity_notification_outbox(recipient_id,kind,severity,mandatory,details,request_id,created_at) VALUES($1,$2,'warning',true,'{\"count\":10}','req_test',now()-$3::interval)", recipient, kind, fmt.Sprintf("%d seconds", int(age.Seconds()))); e != nil {
		t.Fatal(e)
	}
}

func TestDeliverListMarkRead(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		notice(t, s, alice, "login.failures", time.Duration(10-i)*time.Minute)
	}
	notice(t, s, bob, "password.changed", time.Minute)
	if n, e := s.Deliver(ctx); e != nil || n != 6 {
		t.Fatal("deliver", n, e)
	}
	// Re-delivery after a crash between copy and mark must not duplicate.
	if _, e := s.DB.Exec(ctx, "UPDATE identity_notification_outbox SET delivered_at=NULL"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Deliver(ctx); e != nil {
		t.Fatal(e)
	}
	var total int
	s.DB.QueryRow(ctx, "SELECT count(*) FROM notifications").Scan(&total)
	if total != 6 {
		t.Fatal("duplicate delivery", total)
	}

	first, e := s.List(ctx, alice, false, "", 2)
	if e != nil || len(first.Items) != 2 || first.NextCursor == nil || first.UnreadCount != 5 {
		t.Fatal("first page", first, e)
	}
	if first.Items[0].Source != "com.minhanos.account-center" || first.Items[0].Params["count"] != float64(10) || !first.Items[0].Mandatory {
		t.Fatal("notice fields", first.Items[0])
	}
	seen := map[string]bool{}
	cursor := ""
	for {
		page, e := s.List(ctx, alice, false, cursor, 2)
		if e != nil {
			t.Fatal(e)
		}
		for _, n := range page.Items {
			if seen[n.ID] {
				t.Fatal("page overlap", n.ID)
			}
			seen[n.ID] = true
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if len(seen) != 5 {
		t.Fatal("pagination lost items", len(seen))
	}

	// Recipients only see and mark their own notices.
	if updated, _, e := s.MarkRead(ctx, bob, []string{first.Items[0].ID}, nil); e != nil || updated != 0 {
		t.Fatal("cross-user mark", updated, e)
	}
	if updated, unread, e := s.MarkRead(ctx, alice, []string{first.Items[0].ID}, nil); e != nil || updated != 1 || unread != 4 {
		t.Fatal("mark one", updated, unread, e)
	}
	if updated, _, e := s.MarkRead(ctx, alice, []string{first.Items[0].ID}, nil); e != nil || updated != 0 {
		t.Fatal("mark must be idempotent", updated, e)
	}
	unreadPage, _ := s.List(ctx, alice, true, "", 50)
	if len(unreadPage.Items) != 4 {
		t.Fatal("unread filter", len(unreadPage.Items))
	}
	// "Mark all" up to what the user saw leaves a newer notice unread.
	upTo := first.Items[0].CreatedAt
	notice(t, s, alice, "role.changed", -time.Minute)
	s.Deliver(ctx)
	if _, unread, e := s.MarkRead(ctx, alice, nil, &upTo); e != nil || unread != 1 {
		t.Fatal("mark all up to", unread, e)
	}
	if bobPage, _ := s.List(ctx, bob, false, "", 10); bobPage.UnreadCount != 1 || len(bobPage.Items) != 1 {
		t.Fatal("bob inbox", bobPage)
	}
}

func TestInvalidInputAndRetention(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	for _, c := range []string{"x", "bm90LWEtY3Vyc29y", encodeCursor(time.Now(), "not-a-uuid")} {
		if _, e := s.List(ctx, alice, false, c, 10); !errors.Is(e, ErrInvalid) {
			t.Fatal("cursor accepted", c)
		}
	}
	if _, e := s.List(ctx, alice, false, "", MaxPage+1); !errors.Is(e, ErrInvalid) {
		t.Fatal("unbounded page accepted")
	}
	now := time.Now()
	if _, _, e := s.MarkRead(ctx, alice, []string{alice}, &now); !errors.Is(e, ErrInvalid) {
		t.Fatal("ids and upTo together accepted")
	}
	if _, _, e := s.MarkRead(ctx, alice, nil, nil); !errors.Is(e, ErrInvalid) {
		t.Fatal("empty mark accepted")
	}
	notice(t, s, alice, "password.reset", (RetentionDays+1)*24*time.Hour)
	notice(t, s, alice, "password.changed", time.Hour)
	s.Deliver(ctx)
	if e := s.Purge(ctx); e != nil {
		t.Fatal(e)
	}
	page, _ := s.List(ctx, alice, false, "", 10)
	if len(page.Items) != 1 || page.Items[0].Kind != "password.changed" {
		t.Fatal("retention", page.Items)
	}
}
