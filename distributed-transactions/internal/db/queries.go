package db

import (
	"context"
	"database/sql"
	"errors"
)

type TransactionState string

const (
	StatePending   TransactionState = "PENDING"
	StatePrepared  TransactionState = "PREPARED"
	StateCommitted TransactionState = "COMMITTED"
	StateAborted   TransactionState = "ABORTED"
)

var ErrNotFound = errors.New("not found")

type TransactionLog struct {
	ID            int64
	TransactionID string
	Participant   string
	State         TransactionState
	Payload       string
	CreatedAt     string
	UpdatedAt     string
}

func (d *DB) CreateTransactionLog(ctx context.Context, tx *sql.Tx, log *TransactionLog) error {
	query := `INSERT INTO transaction_log (transaction_id, participant, state, payload) VALUES (?, ?, ?, ?)`

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, log.TransactionID, log.Participant, log.State, log.Payload)
	} else {
		_, err = d.ExecContext(ctx, query, log.TransactionID, log.Participant, log.State, log.Payload)
	}
	return err
}

func (d *DB) UpdateTransactionState(ctx context.Context, tx *sql.Tx, transactionID, participant string, state TransactionState) error {
	query := `UPDATE transaction_log SET state = ?, updated_at = datetime('now') WHERE transaction_id = ? AND participant = ?`

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, state, transactionID, participant)
	} else {
		_, err = d.ExecContext(ctx, query, state, transactionID, participant)
	}
	return err
}

func (d *DB) GetTransactionLog(ctx context.Context, transactionID, participant string) (*TransactionLog, error) {
	var log TransactionLog
	err := d.QueryRowContext(ctx,
		`SELECT id, transaction_id, participant, state, payload, created_at, updated_at 
		 FROM transaction_log WHERE transaction_id = ? AND participant = ?`,
		transactionID, participant).
		Scan(&log.ID, &log.TransactionID, &log.Participant, &log.State, &log.Payload, &log.CreatedAt, &log.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &log, nil
}

func (d *DB) GetAllTransactionLogs(ctx context.Context, transactionID string) ([]TransactionLog, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, transaction_id, participant, state, payload, created_at, updated_at 
		 FROM transaction_log WHERE transaction_id = ?`,
		transactionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []TransactionLog
	for rows.Next() {
		var log TransactionLog
		if err := rows.Scan(&log.ID, &log.TransactionID, &log.Participant, &log.State, &log.Payload, &log.CreatedAt, &log.UpdatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (d *DB) GetPendingTransactions(ctx context.Context) ([]TransactionLog, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT DISTINCT transaction_id FROM transaction_log WHERE state NOT IN (?, ?)`,
		StateCommitted, StateAborted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		transactionIDs = append(transactionIDs, id)
	}

	var logs []TransactionLog
	for _, txID := range transactionIDs {
		txLogs, err := d.GetAllTransactionLogs(ctx, txID)
		if err != nil {
			return nil, err
		}
		logs = append(logs, txLogs...)
	}
	return logs, nil
}

func (d *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return d.DB.BeginTx(ctx, nil)
}

func (d *DB) ReserveInventory(ctx context.Context, tx *sql.Tx, itemID string, quantity int) error {
	query := `UPDATE inventory SET quantity = quantity - ? WHERE item_id = ? AND quantity >= ?`

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, quantity, itemID, quantity)
	} else {
		_, err = d.ExecContext(ctx, query, quantity, itemID, quantity)
	}
	return err
}

func (d *DB) ChargePayment(ctx context.Context, tx *sql.Tx, userID string, amount float64) error {
	query := `UPDATE payment_accounts SET balance = balance - ? WHERE user_id = ? AND balance >= ?`

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, amount, userID, amount)
	} else {
		_, err = d.ExecContext(ctx, query, amount, userID, amount)
	}
	return err
}

func (d *DB) ConfirmShipping(ctx context.Context, tx *sql.Tx, userID string) error {
	return nil
}
