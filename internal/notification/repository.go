package notification

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func (r *Repository) MarkParsed(ctx context.Context, tx pgx.Tx, notificationID, transactionID int64, parserName string) error {
	_, err := tx.Exec(ctx, `UPDATE raw_notifications SET status='parsed', transaction_id=$2, parser_name=$3, error_message=NULL WHERE id=$1`, notificationID, transactionID, parserName)
	if err != nil {
		return fmt.Errorf("mark notification parsed: %w", err)
	}
	return nil
}

func (r *Repository) MarkIgnored(ctx context.Context, notificationID int64, parserName string) error {
	_, err := r.db.Exec(ctx, `UPDATE raw_notifications SET status='ignored', transaction_id=NULL, parser_name=$2, error_message=NULL WHERE id=$1`, notificationID, parserName)
	if err != nil {
		return fmt.Errorf("mark notification ignored: %w", err)
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, notificationID int64, reason, parserName string) error {
	_, err := r.db.Exec(ctx, `UPDATE raw_notifications SET status='failed', transaction_id=NULL, parser_name=$2, error_message=$3 WHERE id=$1`, notificationID, parserName, reason)
	if err != nil {
		return fmt.Errorf("mark notification failed: %w", err)
	}
	return nil
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	notification Notification,
) (Notification, error) {
	var created Notification

	err := r.db.QueryRow(
		ctx,
		`
		INSERT INTO raw_notifications (
			source_app,
			title,
			body,
			received_at,
			status,
			raw_payload,
			fingerprint
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING
			id,
			source_app,
			title,
			body,
			received_at,
			status,
			parser_name,
			raw_payload,
			fingerprint,
			error_message,
			created_at
		`,
		notification.SourceApp,
		notification.Title,
		notification.Body,
		notification.ReceivedAt,
		notification.Status,
		notification.RawPayload,
		notification.Fingerprint,
	).Scan(
		&created.ID,
		&created.SourceApp,
		&created.Title,
		&created.Body,
		&created.ReceivedAt,
		&created.Status,
		&created.ParserName,
		&created.RawPayload,
		&created.Fingerprint,
		&created.ErrorMessage,
		&created.CreatedAt,
	)

	if err != nil {
		return Notification{}, fmt.Errorf(
			"create raw notification: %w",
			err,
		)
	}

	return created, nil
}

func (r *Repository) List(ctx context.Context, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			source_app,
			title,
			body,
			received_at,
			status,
			parser_name,
			fingerprint,
			error_message,
			transaction_id,
			created_at
		FROM raw_notifications
		ORDER BY received_at DESC, id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list raw notifications: %w", err)
	}
	defer rows.Close()

	notifications := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		if err := rows.Scan(
			&n.ID,
			&n.SourceApp,
			&n.Title,
			&n.Body,
			&n.ReceivedAt,
			&n.Status,
			&n.ParserName,
			&n.Fingerprint,
			&n.ErrorMessage,
			&n.TransactionID,
			&n.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan raw notification: %w", err)
		}
		notifications = append(notifications, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate raw notifications: %w", err)
	}
	return notifications, nil
}
