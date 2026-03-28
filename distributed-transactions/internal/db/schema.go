package db

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite"
)

const (
	CreateInventoryTable = `
	CREATE TABLE IF NOT EXISTS inventory (
		item_id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		quantity INTEGER NOT NULL DEFAULT 0,
		price REAL NOT NULL DEFAULT 0.0
	);
	`

	CreatePaymentAccountsTable = `
	CREATE TABLE IF NOT EXISTS payment_accounts (
		user_id TEXT PRIMARY KEY,
		balance REAL NOT NULL DEFAULT 0.0
	);
	`

	CreateTransactionLogTable = `
	CREATE TABLE IF NOT EXISTS transaction_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		transaction_id TEXT NOT NULL,
		participant TEXT NOT NULL,
		state TEXT NOT NULL,
		payload TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE(transaction_id, participant)
	);
	CREATE INDEX IF NOT EXISTS idx_tx_state ON transaction_log(state);
	CREATE INDEX IF NOT EXISTS idx_tx_id ON transaction_log(transaction_id);
	`

	CreateCoordinatorTransactionsTable = `
	CREATE TABLE IF NOT EXISTS coordinator_transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		transaction_id TEXT UNIQUE NOT NULL,
		idempotency_key TEXT UNIQUE,
		state TEXT NOT NULL,
		user_id TEXT,
		item_id TEXT,
		quantity INTEGER,
		amount REAL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_coord_tx_state ON coordinator_transactions(state);
	CREATE INDEX IF NOT EXISTS idx_coord_tx_id ON coordinator_transactions(transaction_id);
	CREATE INDEX IF NOT EXISTS idx_coord_tx_idem_key ON coordinator_transactions(idempotency_key);
	`

	CreateIdempotencyKeysTable = `
	CREATE TABLE IF NOT EXISTS idempotency_keys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id TEXT UNIQUE NOT NULL,
		response BLOB NOT NULL,
		expires_at INTEGER NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_idem_message_id ON idempotency_keys(message_id);
	CREATE INDEX IF NOT EXISTS idx_idem_expires_at ON idempotency_keys(expires_at);
	`
)

type DB struct {
	*sql.DB
}

func InitDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (d *DB) InitSchema(ctx context.Context) error {
	tables := []string{
		CreateInventoryTable,
		CreatePaymentAccountsTable,
		CreateTransactionLogTable,
		CreateCoordinatorTransactionsTable,
		CreateIdempotencyKeysTable,
	}

	for _, table := range tables {
		if _, err := d.ExecContext(ctx, table); err != nil {
			return err
		}
	}

	return nil
}
