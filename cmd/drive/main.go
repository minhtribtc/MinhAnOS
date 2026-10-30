package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"minhanos.local/platform/internal/coreprofile"
	"minhanos.local/platform/internal/drive"
	"minhanos.local/platform/internal/storagecore"
	"net/http"
	"os"
	"os/signal"
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
	profile, err := coreprofile.New(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	app := &drive.Server{Profile: profile, AllowPlatformSessions: os.Getenv("ALLOW_PLATFORM_SESSIONS") == "true", Identity: identity, DataDir: env("DRIVE_DATA_DIR", "/data"), StaticDir: env("DRIVE_STATIC_DIR", "/app/web"), AgentSocket: os.Getenv("DRIVE_AGENT_SOCKET"), SecureCookie: os.Getenv("COOKIE_SECURE") == "true", Quota: storagecore.Store{DB: db}}
	app.StartMeasurer(ctx, 30*time.Second)
	server := &http.Server{Addr: env("DRIVE_LISTEN", ":8082"), Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
	}()
	log.Print("MinhAnOS Drive 0.1 listening on ", server.Addr)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
