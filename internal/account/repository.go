package account

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
	rows, err := r.db.Query(
		ctx,
		`
	SELECT
		a.id,
		a.name,
		a.provider,
		a.type,
		a.opening_balance,

		a.opening_balance
		+ COALESCE((
			SELECT SUM(t.amount)
			FROM transactions t
			WHERE
				t.type = 'income'
				AND t.destination_account_id = a.id
		), 0)

		+ COALESCE((
			SELECT SUM(t.amount)
			FROM transactions t
			WHERE
				t.type = 'transfer'
				AND t.destination_account_id = a.id
		), 0)

		- COALESCE((
			SELECT SUM(t.amount)
			FROM transactions t
			WHERE
				t.type = 'expense'
				AND t.source_account_id = a.id
		), 0)

		- COALESCE((
			SELECT SUM(t.amount)
			FROM transactions t
			WHERE
				t.type = 'transfer'
				AND t.source_account_id = a.id
		), 0)

		AS balance,

		a.is_active,
		a.created_at,
		a.updated_at
	FROM accounts a
	ORDER BY a.name
	`,
	)
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
