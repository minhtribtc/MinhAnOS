// identity-admin exposes only status/bounded UUID backfill, never user provisioning or shell execution.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"minhanos.local/platform/internal/identity"
	"os"
	"time"
)

func main() {
	mode := flag.String("mode", "status", "status or backfill")
	batch := flag.Int("batch-size", 250, "1..1000 rows per batch")
	max := flag.Int("max-batches", 1, "1..1000 batches per invocation")
	flag.Parse()
	if (*mode != "status" && *mode != "backfill") || *batch < 1 || *batch > 1000 || *max < 1 || *max > 1000 {
		log.Fatal("invalid bounded identity operation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal("identity database configuration invalid")
	}
	defer db.Close()
	repo := identity.Repository{DB: db}
	var updated int64
	if *mode == "backfill" {
		for i := 0; i < *max; i++ {
			n, err := repo.BackfillBatch(ctx, *batch)
			if err != nil {
				log.Fatal("identity backfill failed; inspect database diagnostics privately")
			}
			updated += n
			if n == 0 {
				break
			}
		}
	}
	status, e := repo.Status(ctx)
	if e != nil {
		log.Fatal("identity schema/status unavailable")
	}
	json.NewEncoder(os.Stdout).Encode(struct {
		Updated int64                   `json:"updated"`
		Status  identity.BackfillStatus `json:"status"`
	}{updated, status})
}
