package ingestor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresBlockCache stores raw blocks by height for reuse across runs. Rows are
// validated against the current chain tip hash from the API before reuse (reorg-safe).
type PostgresBlockCache struct {
	pool *pgxpool.Pool
}

// NewPostgresBlockCache opens a pool and ensures the cache table exists.
func NewPostgresBlockCache(ctx context.Context, connString string) (*PostgresBlockCache, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	const ddl = `
CREATE TABLE IF NOT EXISTS ingested_blocks (
	height BIGINT PRIMARY KEY,
	block_hash TEXT NOT NULL,
	block_json JSONB NOT NULL
)`
	if _, err := pool.Exec(ctx, ddl); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create ingested_blocks: %w", err)
	}
	return &PostgresBlockCache{pool: pool}, nil
}

// Close releases the pool.
func (p *PostgresBlockCache) Close() {
	if p != nil && p.pool != nil {
		p.pool.Close()
	}
}

// Get returns a cached block at height if present.
func (p *PostgresBlockCache) Get(ctx context.Context, height int) (hash string, blk Block, ok bool, err error) {
	if p == nil || p.pool == nil {
		return "", Block{}, false, nil
	}
	var storedHash string
	var raw []byte
	q := `SELECT block_hash, block_json FROM ingested_blocks WHERE height = $1`
	err = p.pool.QueryRow(ctx, q, height).Scan(&storedHash, &raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", Block{}, false, nil
		}
		return "", Block{}, false, err
	}
	fmt.Printf("Cache hit for block height %d\n", height)
	var b Block
	if err := json.Unmarshal(raw, &b); err != nil {
		return "", Block{}, false, fmt.Errorf("unmarshal cached block height %d: %w", height, err)
	}
	return storedHash, b, true, nil
}

// Put inserts or replaces the block at height.
func (p *PostgresBlockCache) Put(ctx context.Context, height int, hash string, blk Block) error {
	if p == nil || p.pool == nil {
		return nil
	}
	raw, err := json.Marshal(blk)
	if err != nil {
		return err
	}
	const q = `
INSERT INTO ingested_blocks (height, block_hash, block_json)
VALUES ($1, $2, $3::jsonb)
ON CONFLICT (height) DO UPDATE SET
	block_hash = EXCLUDED.block_hash,
	block_json = EXCLUDED.block_json`
	_, err = p.pool.Exec(ctx, q, height, hash, raw)
	return err
}
