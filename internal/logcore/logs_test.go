package logcore

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

const admin = "11111111-1111-4111-8111-111111111111"

func fixture(t *testing.T) Store {
	t.Helper()
	dsn := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated log database not configured")
	}
	ctx := context.Background()
	root, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("logs_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{name}.Sanitize()
	if _, e = root.Exec(ctx, "CREATE SCHEMA "+quoted); e != nil {
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
	t.Cleanup(func() { db.Close(); root.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); root.Close() })
	// Stand-ins for the tables owned by identity and storage (only the columns Log Center reads).
	for _, q := range []string{
		"CREATE TABLE users(id text PRIMARY KEY,public_id uuid,username text)",
		"CREATE TABLE identity_admin_events(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),actor_id uuid,action text NOT NULL,target_id text,outcome text NOT NULL,request_id text NOT NULL,correlation_id text,details jsonb NOT NULL DEFAULT '{}',occurred_at timestamptz NOT NULL DEFAULT now(),organization_id uuid)",
		"CREATE TABLE storage_events(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),actor_id uuid,action text NOT NULL,target text,request_id text NOT NULL,correlation_id text,details jsonb NOT NULL DEFAULT '{}',occurred_at timestamptz NOT NULL DEFAULT now(),organization_id uuid)",
		"CREATE TABLE files_events(id uuid PRIMARY KEY DEFAULT gen_random_uuid(),actor_id uuid,action text NOT NULL,root text NOT NULL,share_key text,request_id text NOT NULL,correlation_id text,details jsonb NOT NULL DEFAULT '{}',occurred_at timestamptz NOT NULL DEFAULT now(),organization_id uuid,outcome text NOT NULL DEFAULT 'success')",
		"INSERT INTO users VALUES('1','" + admin + "','root')",
	} {
		if _, e = db.Exec(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	return Store{DB: db}
}

func TestEventsMergeFilterAndPage(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		outcome := "success"
		if i == 1 {
			outcome = "denied"
		}
		if _, e := s.DB.Exec(ctx, "INSERT INTO identity_admin_events(actor_id,action,target_id,outcome,request_id,details,occurred_at) VALUES($1,'identity.user.update',$2,$3,'req_i',jsonb_build_object('reasonCode','x'),now()-make_interval(mins=>$4))", admin, admin, outcome, i*2); e != nil {
			t.Fatal(e)
		}
		if _, e := s.DB.Exec(ctx, "INSERT INTO storage_events(actor_id,action,target,request_id,occurred_at) VALUES($1,'storage.share.create','media','req_s',now()-make_interval(mins=>$2))", admin, i*2+1); e != nil {
			t.Fatal(e)
		}
	}
	all, e := s.Events(ctx, Filter{Limit: 3})
	if e != nil || len(all.Items) != 3 || all.NextCursor == nil {
		t.Fatal("first page", all, e)
	}
	if all.Items[0].Source != "identity" || all.Items[1].Source != "storage" || all.Items[0].Actor == nil || all.Items[0].Actor.Username != "root" || all.Items[0].TargetName == nil || *all.Items[0].TargetName != "root" {
		t.Fatal("merged order and names", all.Items[0], all.Items[1])
	}
	seen, cursor := 0, ""
	for {
		page, e := s.Events(ctx, Filter{Limit: 3, Cursor: cursor})
		if e != nil {
			t.Fatal(e)
		}
		for i := 1; i < len(page.Items); i++ {
			if page.Items[i].OccurredAt.After(page.Items[i-1].OccurredAt) {
				t.Fatal("not newest first")
			}
		}
		seen += len(page.Items)
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if seen != 8 {
		t.Fatal("pagination", seen)
	}
	if _, e := s.DB.Exec(ctx, "INSERT INTO files_events(actor_id,action,root,share_key,request_id,occurred_at) VALUES($1,'files.upload','share','media','req_f',now()-interval '30 minutes')", admin); e != nil {
		t.Fatal(e)
	}
	files, e := s.Events(ctx, Filter{Source: "files", Limit: 50})
	if e != nil || len(files.Items) != 1 || files.Items[0].Target == nil || *files.Items[0].Target != "media" || files.Items[0].Details["root"] != "share" {
		t.Fatal("files source", files, e)
	}
	// Files events carry their outcome and path (ADR 0012); actor and text filters reach every source.
	if _, e := s.DB.Exec(ctx, `INSERT INTO files_events(actor_id,action,root,request_id,outcome,details) VALUES($1,'files.list','personal','req_g','denied','{"path":"Ke toan/2026","code":"ACCESS_DENIED"}')`, admin); e != nil {
		t.Fatal(e)
	}
	if got, e := s.Events(ctx, Filter{Source: "files", Outcome: "denied", Limit: 50}); e != nil || len(got.Items) != 1 || got.Items[0].Target == nil || *got.Items[0].Target != "Ke toan/2026" {
		t.Fatal("files outcome and path", got, e)
	}
	if got, _ := s.Events(ctx, Filter{Query: "KE TOAN", Limit: 50}); len(got.Items) != 1 {
		t.Fatal("text filter", got)
	}
	if got, _ := s.Events(ctx, Filter{Source: "files", Actor: "nobody", Limit: 50}); len(got.Items) != 0 {
		t.Fatal("actor filter", got)
	}
	deniedAll, _ := s.Events(ctx, Filter{Outcome: "denied", Limit: 50})
	if len(deniedAll.Items) != 2 {
		t.Fatal("denied across sources", len(deniedAll.Items))
	}
	storage, _ := s.Events(ctx, Filter{Source: "storage", Limit: 50})
	denied, _ := s.Events(ctx, Filter{Outcome: "denied", Source: "identity", Limit: 50})
	if len(storage.Items) != 4 || len(denied.Items) != 1 || denied.Items[0].Source != "identity" {
		t.Fatal("filters", len(storage.Items), len(denied.Items))
	}
	for _, f := range []Filter{{Limit: 0}, {Limit: MaxPage + 1}, {Limit: 5, Source: "drive"}, {Limit: 5, Outcome: "ok"}, {Limit: 5, Cursor: "%%%"}} {
		if _, e := s.Events(ctx, f); !errors.Is(e, ErrInvalid) {
			t.Fatal("accepted", f)
		}
	}
}
