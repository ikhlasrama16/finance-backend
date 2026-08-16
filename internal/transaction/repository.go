package transaction

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

const transactionColumns = `
	id, type, amount, source_account_id, destination_account_id, category_id,
	description, merchant, parse_status, confidence, source, raw_notification_id,
	occurred_at, created_at, updated_at`

const detachRawNotificationSQL = `
	UPDATE raw_notifications
	SET transaction_id = NULL, status = 'detached', error_message = 'linked transaction deleted manually'
	WHERE transaction_id = $1`

const deleteTransactionSQL = `DELETE FROM transactions WHERE id = $1`

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateParsed(ctx context.Context, tx pgx.Tx, input CreateParsedInput) (Transaction, error) {
	var created Transaction
	err := tx.QueryRow(ctx, `
		INSERT INTO transactions (
			type, amount, source_account_id, destination_account_id, category_id,
			description, merchant, parse_status, confidence, source, raw_notification_id, occurred_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+transactionColumns+`
	`, input.Type, input.Amount, input.SourceAccountID, input.DestinationAccountID, input.CategoryID,
		input.Description, input.Merchant, input.ParseStatus, input.Confidence, "notification",
		input.RawNotificationID, input.OccurredAt).Scan(scanTransaction(&created)...)
	if err != nil {
		return Transaction{}, fmt.Errorf("create parsed transaction: %w", err)
	}
	return created, nil
}

func (r *Repository) Create(ctx context.Context, transaction Transaction) (Transaction, error) {
	var created Transaction
	err := r.db.QueryRow(ctx, `
		INSERT INTO transactions (
			type, amount, source_account_id, destination_account_id, category_id, description, occurred_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+transactionColumns+`
	`, transaction.Type, transaction.Amount, transaction.SourceAccountID, transaction.DestinationAccountID,
		transaction.CategoryID, transaction.Description, transaction.OccurredAt).Scan(scanTransaction(&created)...)
	if err != nil {
		return Transaction{}, fmt.Errorf("create transaction: %w", err)
	}
	return created, nil
}

func (r *Repository) List(ctx context.Context) ([]Transaction, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+transactionColumns+`
		FROM transactions
		ORDER BY occurred_at DESC, id DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	transactions := make([]Transaction, 0)
	for rows.Next() {
		var transaction Transaction
		if err := rows.Scan(scanTransaction(&transaction)...); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactions: %w", err)
	}
	return transactions, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (Transaction, error) {
	var transaction Transaction
	err := r.db.QueryRow(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE id = $1`, id).Scan(scanTransaction(&transaction)...)
	if err != nil {
		return Transaction{}, fmt.Errorf("get transaction: %w", err)
	}
	return transaction, nil
}

func (r *Repository) Update(ctx context.Context, transaction Transaction) (Transaction, error) {
	var updated Transaction
	err := r.db.QueryRow(ctx, `
		UPDATE transactions
		SET category_id = $2, merchant = $3, description = $4, updated_at = NOW()
		WHERE id = $1
		RETURNING `+transactionColumns+`
	`, transaction.ID, transaction.CategoryID, transaction.Merchant, transaction.Description).Scan(scanTransaction(&updated)...)
	if err != nil {
		return Transaction{}, fmt.Errorf("update transaction: %w", err)
	}
	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// This reverses raw_notifications.transaction_id before deleting the
	// transaction, avoiding the circular-looking FK without CASCADE deletion.
	if _, err := tx.Exec(ctx, detachRawNotificationSQL, id); err != nil {
		return fmt.Errorf("detach raw notification: %w", err)
	}
	result, err := tx.Exec(ctx, deleteTransactionSQL, id)
	if err != nil {
		return fmt.Errorf("delete transaction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete transaction: %w", err)
	}
	return nil
}

func scanTransaction(transaction *Transaction) []any {
	return []any{
		&transaction.ID, &transaction.Type, &transaction.Amount, &transaction.SourceAccountID,
		&transaction.DestinationAccountID, &transaction.CategoryID, &transaction.Description,
		&transaction.Merchant, &transaction.ParseStatus, &transaction.Confidence, &transaction.Source,
		&transaction.RawNotificationID, &transaction.OccurredAt, &transaction.CreatedAt, &transaction.UpdatedAt,
	}
}
