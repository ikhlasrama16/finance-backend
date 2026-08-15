package account

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const calculatedBalanceSQL = `
	a.opening_balance
	+ COALESCE((
		SELECT SUM(t.amount)
		FROM transactions t
		WHERE t.type = 'income' AND t.destination_account_id = a.id
	), 0)
	+ COALESCE((
		SELECT SUM(t.amount)
		FROM transactions t
		WHERE t.type = 'transfer' AND t.destination_account_id = a.id
	), 0)
	- COALESCE((
		SELECT SUM(t.amount)
		FROM transactions t
		WHERE t.type = 'expense' AND t.source_account_id = a.id
	), 0)
	- COALESCE((
		SELECT SUM(t.amount)
		FROM transactions t
		WHERE t.type = 'transfer' AND t.source_account_id = a.id
	), 0)`

type Repository struct {
	db *pgxpool.Pool
}

func (r *Repository) GetByName(ctx context.Context, name string) (Account, error) {
	var account Account
	err := r.db.QueryRow(ctx, `
		SELECT id, name, provider, type, opening_balance, is_active, created_at, updated_at
		FROM accounts WHERE LOWER(name) = LOWER($1)
	`, strings.TrimSpace(name)).Scan(
		&account.ID, &account.Name, &account.Provider, &account.Type,
		&account.OpeningBalance, &account.IsActive, &account.CreatedAt, &account.UpdatedAt,
	)
	if err != nil {
		return Account{}, fmt.Errorf("get account by name: %w", err)
	}
	return account, nil
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) List(ctx context.Context) ([]Account, error) {
	rows, err := r.db.Query(ctx, `
	SELECT
		a.id,
		a.name,
		a.provider,
		a.type,
		a.opening_balance,

		`+calculatedBalanceSQL+` AS balance,

		a.is_active,
		a.created_at,
		a.updated_at
	FROM accounts a
	ORDER BY a.name
	`)
	if err != nil {
		return nil, fmt.Errorf("query accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]Account, 0)

	for rows.Next() {
		var account Account

		if err := rows.Scan(
			&account.ID,
			&account.Name,
			&account.Provider,
			&account.Type,
			&account.OpeningBalance,
			&account.Balance,
			&account.IsActive,
			&account.CreatedAt,
			&account.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}

		accounts = append(accounts, account)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounts: %w", err)
	}

	return accounts, nil
}

// Reconcile records a ledger adjustment while holding the account row lock so
// concurrent reconciliation requests cannot calculate from the same balance.
func (r *Repository) Reconcile(ctx context.Context, input ReconcileInput) (Reconciliation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Reconciliation{}, fmt.Errorf("begin reconciliation transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var previousBalance int64
	err = tx.QueryRow(ctx, `
		SELECT `+calculatedBalanceSQL+`
		FROM accounts a
		WHERE a.id = $1
		FOR UPDATE
	`, input.AccountID).Scan(&previousBalance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Reconciliation{}, ErrAccountNotFound
		}
		return Reconciliation{}, fmt.Errorf("load account for reconciliation: %w", err)
	}

	result := Reconciliation{
		AccountID:       input.AccountID,
		PreviousBalance: previousBalance,
		ActualBalance:   input.ActualBalance,
		Difference:      input.ActualBalance - previousBalance,
	}
	if result.Difference == 0 {
		if err := tx.Commit(ctx); err != nil {
			return Reconciliation{}, fmt.Errorf("commit reconciliation transaction: %w", err)
		}
		return result, nil
	}

	adjustment := newReconciliationAdjustment(input.AccountID, result.Difference, input.Description)

	var categoryID int64
	err = tx.QueryRow(ctx, `
		SELECT id FROM categories
		WHERE LOWER(name) = LOWER('Penyesuaian Saldo') AND type = $1
	`, adjustment.CategoryType).Scan(&categoryID)
	if err != nil {
		return Reconciliation{}, fmt.Errorf("load reconciliation category: %w", err)
	}

	var transactionID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO transactions (
			type, amount, source_account_id, destination_account_id, category_id,
			description, merchant, parse_status, confidence, source, raw_notification_id, occurred_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULL, $7, $8, $9, $10, NOW())
		RETURNING id
	`, adjustment.TransactionType, adjustment.Amount, adjustment.SourceAccountID, adjustment.DestinationAccountID, categoryID, adjustment.Description,
		adjustment.ParseStatus, adjustment.Confidence, adjustment.Source, adjustment.RawNotificationID).Scan(&transactionID)
	if err != nil {
		return Reconciliation{}, fmt.Errorf("create reconciliation transaction: %w", err)
	}
	result.TransactionID = &transactionID

	if err := tx.Commit(ctx); err != nil {
		return Reconciliation{}, fmt.Errorf("commit reconciliation transaction: %w", err)
	}
	return result, nil
}

func (r *Repository) Create(
	ctx context.Context,
	input CreateInput,
) (Account, error) {
	var account Account

	err := r.db.QueryRow(
		ctx,
		`
		INSERT INTO accounts (
			name,
			provider,
			type,
			opening_balance
		)
		VALUES ($1, $2, $3, $4)
		RETURNING
			id,
			name,
			provider,
			type,
			opening_balance,
			is_active,
			created_at,
			updated_at
		`,
		input.Name,
		input.Provider,
		input.Type,
		input.OpeningBalance,
	).Scan(
		&account.ID,
		&account.Name,
		&account.Provider,
		&account.Type,
		&account.OpeningBalance,
		&account.IsActive,
		&account.CreatedAt,
		&account.UpdatedAt,
	)

	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}

	return account, nil
}
