package transaction

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (r *Repository) CreateParsed(ctx context.Context, tx pgx.Tx, input CreateParsedInput) (Transaction, error) {
	var created Transaction
	err := tx.QueryRow(ctx, `
		INSERT INTO transactions (
			type, amount, source_account_id, destination_account_id, category_id,
			description, merchant, parse_status, confidence, source, raw_notification_id, occurred_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, type, amount, source_account_id, destination_account_id, category_id,
			description, merchant, parse_status, confidence, source, raw_notification_id,
			occurred_at, created_at, updated_at
	`, input.Type, input.Amount, input.SourceAccountID, input.DestinationAccountID, input.CategoryID,
		input.Description, input.Merchant, input.ParseStatus, input.Confidence, "notification",
		input.RawNotificationID, input.OccurredAt).Scan(
		&created.ID, &created.Type, &created.Amount, &created.SourceAccountID,
		&created.DestinationAccountID, &created.CategoryID, &created.Description,
		&created.Merchant, &created.ParseStatus, &created.Confidence, &created.Source,
		&created.RawNotificationID, &created.OccurredAt, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return Transaction{}, fmt.Errorf("create parsed transaction: %w", err)
	}
	return created, nil
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	tx Transaction,
) (Transaction, error) {
	var created Transaction

	err := r.db.QueryRow(
		ctx,
		`
		INSERT INTO transactions (
			type,
			amount,
			source_account_id,
			destination_account_id,
			category_id,
			description,
			occurred_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			type,
			amount,
			source_account_id,
			destination_account_id,
			category_id,
			description,
			occurred_at,
			created_at,
			updated_at
		`,
		tx.Type,
		tx.Amount,
		tx.SourceAccountID,
		tx.DestinationAccountID,
		tx.CategoryID,
		tx.Description,
		tx.OccurredAt,
	).Scan(
		&created.ID,
		&created.Type,
		&created.Amount,
		&created.SourceAccountID,
		&created.DestinationAccountID,
		&created.CategoryID,
		&created.Description,
		&created.OccurredAt,
		&created.CreatedAt,
		&created.UpdatedAt,
	)

	if err != nil {
		return Transaction{}, fmt.Errorf("create transaction: %w", err)
	}

	return created, nil
}

func (r *Repository) List(
	ctx context.Context,
) ([]Transaction, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			id,
			type,
			amount,
			source_account_id,
			destination_account_id,
			category_id,
			description,
			occurred_at,
			created_at,
			updated_at
		FROM transactions
		ORDER BY occurred_at DESC, id DESC
		LIMIT 100
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	transactions := make([]Transaction, 0)

	for rows.Next() {
		var tx Transaction

		if err := rows.Scan(
			&tx.ID,
			&tx.Type,
			&tx.Amount,
			&tx.SourceAccountID,
			&tx.DestinationAccountID,
			&tx.CategoryID,
			&tx.Description,
			&tx.OccurredAt,
			&tx.CreatedAt,
			&tx.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}

		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactions: %w", err)
	}

	return transactions, nil
}
