// Package coreprofile owns account presentation preferences independently of any shell.
package coreprofile

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log"
	"minhanos.local/platform/internal/apicontract"
	"net/http"
	"time"
)

type Profile struct {
	Locale   string `json:"locale"`
	Theme    string `json:"theme"`
	TimeZone string `json:"timeZone"`
}
type Store struct{ DB *pgxpool.Pool }

func New(ctx context.Context, db *pgxpool.Pool) (*Store, error) {
	tx, e := db.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(681023)"); e != nil {
		return nil, e
	}
	_, e = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS user_profiles(user_id text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,locale text NOT NULL DEFAULT 'vi-VN' CHECK(locale IN ('vi-VN','en-US')),theme text NOT NULL DEFAULT 'light' CHECK(theme IN ('light','dark','system')),time_zone text NOT NULL DEFAULT 'Asia/Bangkok');`)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return &Store{db}, nil
}
func Valid(p Profile) bool {
	if p.Locale != "vi-VN" && p.Locale != "en-US" {
		return false
	}
	if p.Theme != "light" && p.Theme != "dark" && p.Theme != "system" {
		return false
	}
	if len(p.TimeZone) > 100 || (p.TimeZone == "" || p.TimeZone == "Local") {
		return false
	}
	_, e := time.LoadLocation(p.TimeZone)
	return e == nil
}

// Handle must only be called after the host has authenticated the user and checked CSRF/Origin.
func (s *Store) Handle(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fail := func(status int, code string) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(apicontract.Error(status, http.StatusText(status), code))
	}
	p := Profile{"vi-VN", "light", "Asia/Bangkok"}
	if r.Method == "PUT" {
		p = Profile{}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		d.DisallowUnknownFields()
		if d.Decode(&p) != nil || d.Decode(&struct{}{}) != io.EOF || !Valid(p) {
			fail(400, "PROFILE_INVALID")
			return
		}
		_, e := s.DB.Exec(r.Context(), `INSERT INTO user_profiles(user_id,locale,theme,time_zone) VALUES($1,$2,$3,$4) ON CONFLICT(user_id) DO UPDATE SET locale=EXCLUDED.locale,theme=EXCLUDED.theme,time_zone=EXCLUDED.time_zone`, userID, p.Locale, p.Theme, p.TimeZone)
		if e != nil {
			log.Print("profile write: ", e)
			fail(500, "INTERNAL_ERROR")
			return
		}
	} else if r.Method == "GET" {
		e := s.DB.QueryRow(r.Context(), "SELECT locale,theme,time_zone FROM user_profiles WHERE user_id=$1", userID).Scan(&p.Locale, &p.Theme, &p.TimeZone)
		if e == pgx.ErrNoRows {
			var exists bool
			if check := s.DB.QueryRow(r.Context(), "SELECT to_regclass('preferences') IS NOT NULL").Scan(&exists); check == nil && exists {
				var theme string
				if old := s.DB.QueryRow(r.Context(), "SELECT value->>'theme' FROM preferences WHERE user_id=$1", userID).Scan(&theme); old == nil && (theme == "light" || theme == "dark") {
					p.Theme = theme
				}
			}
		}
		if e != nil && e != pgx.ErrNoRows {
			log.Print("profile read: ", e)
			fail(500, "INTERNAL_ERROR")
			return
		}
	} else {
		w.Header().Set("Allow", "GET, PUT")
		fail(405, "METHOD_NOT_ALLOWED")
		return
	}
	json.NewEncoder(w).Encode(p)
}
