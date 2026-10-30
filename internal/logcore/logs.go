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

// Sources lists what Log Center reads. Files' legacy drive_audit is not a source: it holds personal file
// paths, which administrators must not read implicitly (app standard row 17).
var Sources = []Source{
	{ID: "identity", AppID: "com.minhanos.account-center", RetentionDays: days(365)},
	{ID: "storage", AppID: "com.minhanos.storage"},
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
	case "", "identity", "storage":
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
	rows, e := s.DB.Query(ctx, `SELECT x.id::text,x.source,x.occurred_at,x.action,x.outcome,x.actor_id::text,coalesce(a.username,''),x.target,t.username,x.request_id,x.correlation_id,x.details
		FROM (
		 (SELECT e.id,'identity' AS source,e.occurred_at,e.action,e.outcome,e.actor_id,e.target_id AS target,e.request_id,e.correlation_id,e.details FROM identity_admin_events e
		  WHERE $1 IN ('','identity') AND ($2='' OR e.outcome=$2) AND ($3::timestamptz IS NULL OR (e.occurred_at,e.id)<($3,$4::uuid))
		  ORDER BY e.occurred_at DESC,e.id DESC LIMIT $5)
		 UNION ALL
		 (SELECT s.id,'storage',s.occurred_at,s.action,'success',s.actor_id,s.target,s.request_id,s.correlation_id,s.details FROM storage_events s
		  WHERE $1 IN ('','storage') AND $2 IN ('','success') AND ($3::timestamptz IS NULL OR (s.occurred_at,s.id)<($3,$4::uuid))
		  ORDER BY s.occurred_at DESC,s.id DESC LIMIT $5)
		) x
		LEFT JOIN users a ON a.public_id=x.actor_id
		LEFT JOIN users t ON t.public_id::text=x.target
		ORDER BY x.occurred_at DESC,x.id DESC LIMIT $5`, f.Source, f.Outcome, at, id, f.Limit+1)
	if e != nil {
		return Page{}, e
	}
	defer rows.Close()
	page := Page{Items: []Event{}}
	for rows.Next() {
		var ev Event
		var actorID, actorName *string
		if e = rows.Scan(&ev.ID, &ev.Source, &ev.OccurredAt, &ev.Action, &ev.Outcome, &actorID, &actorName, &ev.Target, &ev.TargetName, &ev.RequestID, &ev.CorrelationID, &ev.Details); e != nil {
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
