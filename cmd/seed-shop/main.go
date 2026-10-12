package main

import (
	"context"
	"fmt"
	"os"

	"quality_control/internal/db"
	"quality_control/internal/domain"
	"quality_control/internal/store"

	"github.com/google/uuid"
)

func main() {
	dsn := os.Getenv("QC_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://qc:qc-local-password@127.0.0.1:5433/quality_control?sslmode=disable"
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, dsn); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()

	var admin uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE lower(username) = 'admin'`).Scan(&admin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	result, err := store.New(pool).SeedDemo(ctx, domain.Actor{ID: admin})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(result.Summary)
}
