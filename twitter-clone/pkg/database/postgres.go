package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

func NewPostgresDB(ctx context.Context, databaseURL string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &PostgresDB{pool: pool}, nil
}

func (db *PostgresDB) Close() {
	db.pool.Close()
}

func (db *PostgresDB) Health(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

func (db *PostgresDB) Pool() *pgxpool.Pool {
	return db.pool
}

func (db *PostgresDB) Exec(ctx context.Context, sql string, args ...interface{}) error {
	_, err := db.pool.Exec(ctx, sql, args...)
	return err
}
