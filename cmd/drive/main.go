package main

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"io/fs"
	"log"
	"log/slog"
	"minhanos.local/platform/internal/coreprofile"
	"minhanos.local/platform/internal/drive"
	"minhanos.local/platform/internal/storagecore"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 30; i++ {
		if err = db.Ping(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	identity, err := drive.NewSQLIdentity(ctx, db, os.Getenv("BOOTSTRAP_PASSWORD_FILE"))
	if err != nil {
		log.Fatal(err)
	}
	if err = storagecore.ApplySchema(ctx, db); err != nil {
		log.Fatal(err)
	}
	if err = drive.ApplySchema(ctx, db); err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			if e := drive.RetainAudit(ctx, db); e != nil && ctx.Err() == nil {
				slog.Error("files.audit.retention_failed", "error", e.Error())
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(6 * time.Hour):
			}
		}
	}()
	profile, err := coreprofile.New(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	store := storagecore.Store{DB: db}
	dataDir := env("DRIVE_DATA_DIR", "/data")
	// First-run rule (storage Q2): an install whose legacy Files data directory holds no home retires that
	// homes registration; the first System Admin then creates the share hosting homes in Storage Manager.
	users, err := os.ReadDir(filepath.Join(dataDir, "users"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatal(err)
	}
	if retired, err := store.RetireEmptyLegacyHomes(ctx, len(users) > 0); err != nil {
		log.Fatal(err)
	} else if retired {
		slog.Info("storage.homes.legacy_retired")
	}
	app := &drive.Server{Profile: profile, AllowPlatformSessions: os.Getenv("ALLOW_PLATFORM_SESSIONS") == "true", Identity: identity, DataDir: dataDir, StaticDir: env("DRIVE_STATIC_DIR", "/app/web"), AgentSocket: os.Getenv("DRIVE_AGENT_SOCKET"), SecureCookie: os.Getenv("COOKIE_SECURE") == "true", Quota: store, Shares: store, Units: storagecore.AgentClient{Socket: os.Getenv("STORAGE_AGENT_SOCKET")}}
	app.StartMeasurer(ctx, 30*time.Second)
	server := &http.Server{Addr: env("DRIVE_LISTEN", ":8082"), Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
	}()
	slog.Info("files.listening", "addr", server.Addr)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
