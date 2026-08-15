package importer

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func LoadReferences(ctx context.Context, db queryer) ([]Account, []Category, error) {
	accounts, err := loadAccounts(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	categories, err := loadCategories(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	return accounts, categories, nil
}

func loadAccounts(ctx context.Context, db queryer) ([]Account, error) {
	rows, err := db.Query(ctx, `SELECT id, name FROM accounts WHERE is_active = TRUE ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load accounts: %w", err)
	}
	defer rows.Close()
	var values []Account
	for rows.Next() {
		var value Account
		if err := rows.Scan(&value.ID, &value.Name); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func loadCategories(ctx context.Context, db queryer) ([]Category, error) {
	rows, err := db.Query(ctx, `SELECT id, name, type FROM categories ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	defer rows.Close()
	var values []Category
	for rows.Next() {
		var value Category
		if err := rows.Scan(&value.ID, &value.Name, &value.Type); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func EnsureMissingCategories(ctx context.Context, tx pgx.Tx, rows []LegacyRow) error {
	existing, err := loadCategories(ctx, tx)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(existing))
	for _, value := range existing {
		known[categoryKey(value.Name, value.Type)] = true
	}
	for _, row := range rows {
		typ := strings.ToUpper(strings.TrimSpace(row.Type))
		categoryType := ""
		switch typ {
		case "EXPENSE":
			categoryType = "expense"
		case "INCOME":
			categoryType = "income"
		default:
			continue
		}
		name := strings.TrimSpace(row.Category)
		if name == "" {
			if categoryType == "expense" {
				name = "Belum Dikategorikan"
			} else {
				name = "Pemasukan"
			}
		}
		key := categoryKey(name, categoryType)
		if known[key] {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO categories (name, type) VALUES ($1, $2) ON CONFLICT (LOWER(name), type) DO NOTHING`, name, categoryType); err != nil {
			return fmt.Errorf("create missing category %q (%s): %w", name, categoryType, err)
		}
		known[key] = true
	}
	return nil
}

func Insert(ctx context.Context, tx pgx.Tx, value Transaction) (bool, error) {
	command, err := tx.Exec(ctx, `
		INSERT INTO transactions (
			legacy_id, type, amount, source_account_id, destination_account_id, category_id,
			merchant, description, parse_status, confidence, source, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'import',$11)
		ON CONFLICT (legacy_id) WHERE legacy_id IS NOT NULL DO NOTHING
	`, value.LegacyID, value.Type, value.Amount, value.SourceAccountID, value.DestinationAccount, value.CategoryID, value.Merchant, value.Description, value.ParseStatus, value.Confidence, value.OccurredAt)
	if err != nil {
		return false, fmt.Errorf("insert legacy transaction %s: %w", value.LegacyID, err)
	}
	return command.RowsAffected() == 1, nil
}
