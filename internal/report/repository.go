package report

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

const loadTransactionsSQL = `
	SELECT t.type, t.amount, t.source, t.merchant, COALESCE(c.name, ''), t.occurred_at
	FROM transactions t
	LEFT JOIN categories c ON c.id = t.category_id
	WHERE t.occurred_at >= $1 AND t.occurred_at < $2
`

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) LoadTransactions(ctx context.Context, start, endExclusive time.Time) ([]TransactionRecord, error) {
	rows, err := r.db.Query(ctx, loadTransactionsSQL, start, endExclusive)
	if err != nil {
		return nil, fmt.Errorf("query report transactions: %w", err)
	}
	defer rows.Close()
	records := make([]TransactionRecord, 0)
	for rows.Next() {
		var record TransactionRecord
		var merchant *string
		if err := rows.Scan(&record.Type, &record.Amount, &record.Source, &merchant, &record.CategoryName, &record.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan report transaction: %w", err)
		}
		if merchant != nil {
			record.Merchant = *merchant
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report transactions: %w", err)
	}
	return records, nil
}

func (r *Repository) FindCache(ctx context.Context, period Period, summaryHash, model string) (CacheEntry, bool, error) {
	var entry CacheEntry
	err := r.db.QueryRow(ctx, `
		SELECT content, model, created_at
		FROM ai_reports
		WHERE period_type = $1 AND period_start = $2 AND period_end = $3
			AND summary_hash = $4 AND model = $5 AND status = 'complete'
		ORDER BY created_at DESC
		LIMIT 1
	`, period.Type, period.StartDate(), period.EndDate(), summaryHash, model).Scan(&entry.Content, &entry.Model, &entry.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CacheEntry{}, false, nil
	}
	if err != nil {
		return CacheEntry{}, false, fmt.Errorf("find AI report cache: %w", err)
	}
	return entry, true, nil
}

func (r *Repository) SaveCache(ctx context.Context, period Period, summaryHash, content, model string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_reports (period_type, period_start, period_end, summary_hash, content, model, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'complete')
		ON CONFLICT (period_type, period_start, period_end, summary_hash, model) DO NOTHING
	`, period.Type, period.StartDate(), period.EndDate(), summaryHash, content, model)
	if err != nil {
		return fmt.Errorf("save AI report cache: %w", err)
	}
	return nil
}
