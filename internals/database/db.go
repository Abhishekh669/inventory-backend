package database

import (
	"context"
	"fmt"
	"inventory/internals/config"
	"log"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pgPool       *pgxpool.Pool
	pgOnce       sync.Once
	pgConnectErr error

	// PostgreSQL identifier validation regex (optional use)
	pgIdentifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// Acquire a single PostgreSQL connection (REMEMBER to Release it)
func GetPostgresConn(ctx context.Context) (*pgxpool.Conn, error) {
	pool, err := GetPostgresPool()
	if err != nil {
		return nil, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection from pool: %w", err)
	}

	return conn, nil
}

// GetPostgresPool initializes and returns the singleton pool
func GetPostgresPool() (*pgxpool.Pool, error) {
	pgOnce.Do(func() {
		// Parse config
		pgConfig, err := pgxpool.ParseConfig(config.AppConfig.PostgressURL)
		if err != nil {
			pgConnectErr = fmt.Errorf("failed to parse PostgreSQL config: %w", err)
			return
		}

		pgConfig.ConnConfig.Config.RuntimeParams["sslmode"] = "require"

		// Pool settings (safe defaults)
		pgConfig.MaxConns = 20
		pgConfig.MinConns = 2
		pgConfig.MaxConnLifetime = time.Hour
		pgConfig.MaxConnIdleTime = 30 * time.Minute
		pgConfig.HealthCheckPeriod = time.Minute

		// Create pool
		pool, err := pgxpool.NewWithConfig(context.Background(), pgConfig)
		if err != nil {
			pgConnectErr = fmt.Errorf("failed to create PostgreSQL pool: %w", err)
			return
		}

		// Verify connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			pgConnectErr = fmt.Errorf("PostgreSQL ping failed: %w", err)
			return
		}

		pgPool = pool
		log.Println("✅ PostgreSQL connected successfully")
	})

	return pgPool, pgConnectErr
}

// InitializeDatabase initializes DB and runs migrations
func InitializeDatabase() error {
	log.Println("Initializing database connections...")
	log.Println("🐘 Connecting to PostgreSQL...")

	postgresPool, err := GetPostgresPool()
	if err != nil {
		return fmt.Errorf("PostgreSQL connection failed: %w", err)
	}

	if err := CreatePostgresTables(context.Background(), postgresPool); err != nil {
		return fmt.Errorf("error creating PostgreSQL tables: %w", err)
	}

	return nil
}

// ClosePostgres gracefully closes the pool
func ClosePostgres() {
	if pgPool != nil {
		pgPool.Close()
		log.Println("🛑 PostgreSQL pool closed")
	}
}
