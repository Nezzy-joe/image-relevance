package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/config"
	"github.com/Nezzy-joe/image-relevance/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	developmentTenantName = "image-relevance development tenant"
	seedTenantLockID      = int64(0x494d52454c4e5459)
	schemaWaitTimeout     = 30 * time.Second
)

func main() {
	fmt.Println("Starting image-relevance development tenant seed.")
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Seed failed: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration failed: %w", err)
	}
	if cfg.AppEnv != "development" {
		return errors.New("refusing to seed a development tenant unless APP_ENV=development")
	}

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), cfg.DatabaseConnect)
	pool, err := database.OpenPool(connectCtx, database.PoolConfig{
		URL:      cfg.DatabaseURL,
		MaxConns: cfg.DatabaseMaxConns,
	})
	cancelConnect()
	if err != nil {
		return errors.New("database connection failed; check local PostgreSQL availability and DATABASE_URL")
	}
	defer pool.Close()

	schemaCtx, cancelSchema := context.WithTimeout(context.Background(), schemaWaitTimeout)
	defer cancelSchema()
	if err := waitForTenantSchema(schemaCtx, pool); err != nil {
		return errors.New("Phase 1 tenants migration is not ready; start the Compose application and check API startup")
	}

	created, err := seedDevelopmentTenant(schemaCtx, pool)
	if err != nil {
		return err
	}
	if created {
		fmt.Println("Seed complete: created the development tenant.")
	} else {
		fmt.Println("Seed complete: development tenant already exists; no changes made.")
	}
	return nil
}

func waitForTenantSchema(ctx context.Context, pool *pgxpool.Pool) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		var available bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass('public.tenants') IS NOT NULL").Scan(&available); err == nil && available {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func seedDevelopmentTenant(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, errors.New("could not begin tenant seed transaction")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", seedTenantLockID); err != nil {
		return false, errors.New("could not acquire tenant seed lock")
	}

	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM tenants WHERE name = $1)", developmentTenantName).Scan(&exists); err != nil {
		return false, errors.New("could not check for the development tenant")
	}
	if exists {
		if err := tx.Commit(ctx); err != nil {
			return false, errors.New("could not finish tenant seed transaction")
		}
		return false, nil
	}

	if _, err := tx.Exec(ctx, "INSERT INTO tenants (name, status) VALUES ($1, 'active')", developmentTenantName); err != nil {
		return false, errors.New("could not create the development tenant")
	}
	if err := tx.Commit(ctx); err != nil {
		return false, errors.New("could not finish tenant seed transaction")
	}
	return true, nil
}
