package db

import (
	"context"
)

type SeedData struct {
	Inventory       []InventoryItem
	PaymentAccounts []PaymentAccount
}

type InventoryItem struct {
	ItemID   string
	Name     string
	Quantity int
	Price    float64
}

type PaymentAccount struct {
	UserID  string
	Balance float64
}

func DefaultSeedData() SeedData {
	return SeedData{
		Inventory: []InventoryItem{
			{ItemID: "laptop-001", Name: "MacBook Pro 14", Quantity: 10, Price: 1999.99},
			{ItemID: "phone-001", Name: "iPhone 15 Pro", Quantity: 25, Price: 999.99},
			{ItemID: "tablet-001", Name: "iPad Pro 12.9", Quantity: 15, Price: 1099.99},
			{ItemID: "watch-001", Name: "Apple Watch Ultra", Quantity: 30, Price: 799.99},
			{ItemID: "earbuds-001", Name: "AirPods Pro 2", Quantity: 50, Price: 249.99},
		},
		PaymentAccounts: []PaymentAccount{
			{UserID: "user-001", Balance: 5000.00},
			{UserID: "user-002", Balance: 3000.00},
			{UserID: "user-003", Balance: 10000.00},
		},
	}
}

func (d *DB) Seed(ctx context.Context, data SeedData) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, item := range data.Inventory {
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO inventory (item_id, name, quantity, price) VALUES (?, ?, ?, ?)`,
			item.ItemID, item.Name, item.Quantity, item.Price)
		if err != nil {
			return err
		}
	}

	for _, acc := range data.PaymentAccounts {
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO payment_accounts (user_id, balance) VALUES (?, ?)`,
			acc.UserID, acc.Balance)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) SeedInventory(ctx context.Context, items []InventoryItem) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, item := range items {
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO inventory (item_id, name, quantity, price) VALUES (?, ?, ?, ?)`,
			item.ItemID, item.Name, item.Quantity, item.Price)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) SeedPaymentAccounts(ctx context.Context, accounts []PaymentAccount) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, acc := range accounts {
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO payment_accounts (user_id, balance) VALUES (?, ?)`,
			acc.UserID, acc.Balance)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) ClearAll(ctx context.Context) error {
	tables := []string{"inventory", "payment_accounts", "transaction_log"}
	for _, table := range tables {
		if _, err := d.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) GetInventory(ctx context.Context, itemID string) (*InventoryItem, error) {
	var item InventoryItem
	err := d.QueryRowContext(ctx,
		`SELECT item_id, name, quantity, price FROM inventory WHERE item_id = ?`, itemID).
		Scan(&item.ItemID, &item.Name, &item.Quantity, &item.Price)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (d *DB) GetPaymentAccount(ctx context.Context, userID string) (*PaymentAccount, error) {
	var acc PaymentAccount
	err := d.QueryRowContext(ctx,
		`SELECT user_id, balance FROM payment_accounts WHERE user_id = ?`, userID).
		Scan(&acc.UserID, &acc.Balance)
	if err != nil {
		return nil, err
	}
	return &acc, nil
}
