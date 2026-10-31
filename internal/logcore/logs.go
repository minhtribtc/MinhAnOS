// Package logcore is the Log Center backend (minimal v1): one read-only, newest-first view over the
// append-only audit stores of each module. It owns no data; every source keeps its own table, retention
// and write path. Application/system logs (journald, containers) are not sources yet.
package logcore

import (
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const MaxPage = 100

var (
	ErrInvalid = errors.New("invalid log request")
	uuidRE     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// Source is one audit store. RetentionDays nil means the source has no retention policy yet.
type Source struct {
	ID            string `json:"id"`
	AppID         string `json:"appId"`
	RetentionDays *int   `json:"retentionDays"`
}

func days(n int) *int { return &n }

// Sources lists what Log Center reads. Files writes files_events without personal paths (app standard
// row 17); its legacy drive_audit table holds personal file paths and is not a source.
var Sources = []Source{
	{ID: "identity", AppID: "com.minhanos.account-center", RetentionDays: days(365)},
	{ID: "storage", AppID: "com.minhanos.storage"},
	{ID: "files", AppID: "com.minhanos.files", RetentionDays: days(365)},
}

type Person struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Event struct {
	ID            string         `json:"id"`
	Source        string         `json:"source"`
	OccurredAt    time.Time      `json:"occurredAt"`
	Action        string         `json:"action"`
	Outcome       string         `json:"outcome"`
	Actor         *Person        `json:"actor"`
	Target        *string        `json:"target"`
	TargetName    *string        `json:"targetName"`
	RequestID     string         `json:"requestId"`
	CorrelationID *string        `json:"correlationId"`
	Details       map[string]any `json:"details"`
	// OrganizationID is the organization the event belongs to (ADR 0008); null for platform events.
	OrganizationID *string `json:"organizationId"`
}

type Page struct {
	Items      []Event `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

type Filter struct {
	Source  string // "" = all
	Outcome string // "" = all
	Cursor  string
	Limit   int
	// Organizations limits events to these organizations (organization admins); nil = every event (Root).
	Organizations []string
	// Actor is a username ("" = everyone); Query finds text in the target and details, such as a Files path.
	Actor string
	Query string
}

type Store struct{ DB *pgxpool.Pool }

func encodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(at.UnixMicro(), 10) + "." + id))
}

func decodeCursor(c string) (*time.Time, *string, error) {
	if c == "" {
		return nil, nil, nil
	}
	raw, e := base64.RawURLEncoding.DecodeString(c)
	if e != nil {
		return nil, nil, ErrInvalid
	}
	micros, id, ok := strings.Cut(string(raw), ".")
	n, e := strconv.ParseInt(micros, 10, 64)
	if !ok || e != nil || !uuidRE.MatchString(id) {
		return nil, nil, ErrInvalid
	}
	at := time.UnixMicro(n)
	return &at, &id, nil
}

// Events reads one page across sources. Each source is limited before the union, so a request never scans
// more than two pages of rows (app standard row 25).
func (s Store) Events(ctx context.Context, f Filter) (Page, error) {
	if f.Limit < 1 || f.Limit > MaxPage {
		return Page{}, ErrInvalid
	}
	switch f.Source {
	case "", "identity", "storage", "files":
	default:
		return Page{}, ErrInvalid
	}
	switch f.Outcome {
	case "", "success", "denied", "failure":
	default:
		return Page{}, ErrInvalid
	}
	at, id, e := decodeCursor(f.Cursor)
	if e != nil {
		return Page{}, e
	}
	rows, e := s.DB.Query(ctx, `SELECT x.id::text,x.source,x.occurred_at,x.action,x.outcome,x.actor_id::text,coalesce(a.username,''),x.target,t.username,x.request_id,x.correlation_id,x.details,x.organization_id::text
		FROM (
		 (SELECT e.id,'identity' AS source,e.occurred_at,e.action,e.outcome,e.actor_id,e.target_id AS target,e.request_id,e.correlation_id,e.details,e.organization_id FROM identity_admin_events e
		  WHERE $1 IN ('','identity') AND ($6::uuid[] IS NULL OR e.organization_id=ANY($6::uuid[])) AND ($2='' OR e.outcome=$2) AND ($7='' OR e.actor_id=(SELECT public_id FROM users WHERE username=$7)) AND ($8='' OR strpos(lower(e.details::text||' '||coalesce(e.target_id::text,'')),lower($8))>0) AND ($3::timestamptz IS NULL OR (e.occurred_at,e.id)<($3,$4::uuid))
		  ORDER BY e.occurred_at DESC,e.id DESC LIMIT $5)
		 UNION ALL
		 (SELECT s.id,'storage',s.occurred_at,s.action,'success',s.actor_id,s.target,s.request_id,s.correlation_id,s.details,s.organization_id FROM storage_events s
		  WHERE $1 IN ('','storage') AND ($6::uuid[] IS NULL OR s.organization_id=ANY($6::uuid[])) AND $2 IN ('','success') AND ($7='' OR s.actor_id=(SELECT public_id FROM users WHERE username=$7)) AND ($8='' OR strpos(lower(s.details::text||' '||coalesce(s.target,'')),lower($8))>0) AND ($3::timestamptz IS NULL OR (s.occurred_at,s.id)<($3,$4::uuid))
		  ORDER BY s.occurred_at DESC,s.id DESC LIMIT $5)
		 UNION ALL
		 (SELECT f.id,'files',f.occurred_at,f.action,f.outcome,f.actor_id,coalesce(f.details->>'path',f.share_key,f.root),f.request_id,f.correlation_id,f.details||jsonb_build_object('root',f.root),f.organization_id FROM files_events f
		  WHERE $1 IN ('','files') AND ($6::uuid[] IS NULL OR f.organization_id=ANY($6::uuid[])) AND ($2='' OR f.outcome=$2) AND ($7='' OR f.actor_id=(SELECT public_id FROM users WHERE username=$7)) AND ($8='' OR strpos(lower(f.details::text||' '||coalesce(f.share_key,'')),lower($8))>0) AND ($3::timestamptz IS NULL OR (f.occurred_at,f.id)<($3,$4::uuid))
		  ORDER BY f.occurred_at DESC,f.id DESC LIMIT $5)
		) x
		LEFT JOIN users a ON a.public_id=x.actor_id
		LEFT JOIN users t ON t.public_id::text=x.target
		ORDER BY x.occurred_at DESC,x.id DESC LIMIT $5`, f.Source, f.Outcome, at, id, f.Limit+1, f.Organizations, f.Actor, f.Query)
	if e != nil {
		return Page{}, e
	}
	defer rows.Close()
	page := Page{Items: []Event{}}
	for rows.Next() {
		var ev Event
		var actorID, actorName *string
		if e = rows.Scan(&ev.ID, &ev.Source, &ev.OccurredAt, &ev.Action, &ev.Outcome, &actorID, &actorName, &ev.Target, &ev.TargetName, &ev.RequestID, &ev.CorrelationID, &ev.Details, &ev.OrganizationID); e != nil {
			return Page{}, e
		}
		if actorID != nil {
			ev.Actor = &Person{ID: *actorID, Username: *actorName}
		}
		page.Items = append(page.Items, ev)
	}
	if e = rows.Err(); e != nil {
		return Page{}, e
	}
	if len(page.Items) > f.Limit {
		page.Items = page.Items[:f.Limit]
		last := page.Items[f.Limit-1]
		next := encodeCursor(last.OccurredAt, last.ID)
		page.NextCursor = &next
	}
	return page, nil
}
