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

func (r *Repository) LoadAggregatedStatistics(ctx context.Context, start, endExclusive time.Time) (Statistics, error) {
	statistics := Statistics{
		ExpenseByCategory: make([]CategoryTotal, 0),
		IncomeByCategory:  make([]CategoryTotal, 0),
		TopMerchants:      make([]MerchantTotal, 0),
		TopIncomeSources:  make([]MerchantTotal, 0),
	}
	err := r.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE source <> 'reconcile' AND type = 'income'), 0),
			COALESCE(SUM(amount) FILTER (WHERE source <> 'reconcile' AND type = 'expense'), 0),
			COUNT(*) FILTER (WHERE source <> 'reconcile'),
			COUNT(*) FILTER (WHERE source <> 'reconcile' AND type = 'expense'),
			COUNT(*) FILTER (WHERE source <> 'reconcile' AND type = 'transfer'),
			COALESCE(SUM(CASE WHEN source = 'reconcile' AND type = 'income' THEN amount WHEN source = 'reconcile' AND type = 'expense' THEN -amount ELSE 0 END), 0)
		FROM transactions
		WHERE occurred_at >= $1 AND occurred_at < $2
	`, start, endExclusive).Scan(
		&statistics.Summary.Income, &statistics.Summary.Expense, &statistics.Summary.TransactionCount,
		&statistics.Summary.ExpenseTransactionCount, &statistics.Summary.TransferCount, &statistics.Summary.ReconciliationAdjustment,
	)
	if err != nil {
		return Statistics{}, fmt.Errorf("aggregate report summary: %w", err)
	}
	statistics.Summary.NetCashflow = statistics.Summary.Income - statistics.Summary.Expense
	days := int(endExclusive.Sub(start).Hours() / 24)
	if days > 0 {
		statistics.Summary.AverageDailyExpense = statistics.Summary.Expense / int64(days)
	}

	// 1. Expense by category
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(NULLIF(BTRIM(c.name), ''), 'Belum Dikategorikan'), SUM(t.amount)
		FROM transactions t LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.occurred_at >= $1 AND t.occurred_at < $2 AND t.source <> 'reconcile' AND t.type = 'expense'
		GROUP BY 1 ORDER BY 2 DESC, 1 ASC
	`, start, endExclusive)
	if err != nil {
		return Statistics{}, fmt.Errorf("aggregate report categories: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var value CategoryTotal
		if err := rows.Scan(&value.Category, &value.Amount); err != nil {
			return Statistics{}, fmt.Errorf("scan report category: %w", err)
		}
		if statistics.Summary.Expense > 0 {
			value.Percentage = float64(value.Amount) * 100 / float64(statistics.Summary.Expense)
		}
		statistics.ExpenseByCategory = append(statistics.ExpenseByCategory, value)
	}
	if err := rows.Err(); err != nil {
		return Statistics{}, fmt.Errorf("iterate report categories: %w", err)
	}
	rows.Close()

	// 2. Income by category
	incomeRows, err := r.db.Query(ctx, `
		SELECT COALESCE(NULLIF(BTRIM(c.name), ''), 'Belum Dikategorikan'), SUM(t.amount)
		FROM transactions t LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.occurred_at >= $1 AND t.occurred_at < $2 AND t.source <> 'reconcile' AND t.type = 'income'
		GROUP BY 1 ORDER BY 2 DESC, 1 ASC
	`, start, endExclusive)
	if err != nil {
		return Statistics{}, fmt.Errorf("aggregate report income categories: %w", err)
	}
	defer incomeRows.Close()
	for incomeRows.Next() {
		var value CategoryTotal
		if err := incomeRows.Scan(&value.Category, &value.Amount); err != nil {
			return Statistics{}, fmt.Errorf("scan report income category: %w", err)
		}
		if statistics.Summary.Income > 0 {
			value.Percentage = float64(value.Amount) * 100 / float64(statistics.Summary.Income)
		}
		statistics.IncomeByCategory = append(statistics.IncomeByCategory, value)
	}
	if err := incomeRows.Err(); err != nil {
		return Statistics{}, fmt.Errorf("iterate report income categories: %w", err)
	}
	incomeRows.Close()

	// 3. Top expense merchants
	merchantRows, err := r.db.Query(ctx, `
		SELECT BTRIM(merchant), SUM(amount), COUNT(*)
		FROM transactions
		WHERE occurred_at >= $1 AND occurred_at < $2 AND source <> 'reconcile' AND type = 'expense' AND NULLIF(BTRIM(merchant), '') IS NOT NULL
		GROUP BY 1 ORDER BY 2 DESC, 3 DESC, 1 ASC LIMIT 5
	`, start, endExclusive)
	if err != nil {
		return Statistics{}, fmt.Errorf("aggregate report merchants: %w", err)
	}
	defer merchantRows.Close()
	for merchantRows.Next() {
		var value MerchantTotal
		if err := merchantRows.Scan(&value.Merchant, &value.Amount, &value.TransactionCount); err != nil {
			return Statistics{}, fmt.Errorf("scan report merchant: %w", err)
		}
		statistics.TopMerchants = append(statistics.TopMerchants, value)
	}
	if err := merchantRows.Err(); err != nil {
		return Statistics{}, fmt.Errorf("iterate report merchants: %w", err)
	}
	merchantRows.Close()

	// 4. Top income sources
	incomeSourceRows, err := r.db.Query(ctx, `
		SELECT BTRIM(merchant), SUM(amount), COUNT(*)
		FROM transactions
		WHERE occurred_at >= $1 AND occurred_at < $2 AND source <> 'reconcile' AND type = 'income' AND NULLIF(BTRIM(merchant), '') IS NOT NULL
		GROUP BY 1 ORDER BY 2 DESC, 3 DESC, 1 ASC LIMIT 5
	`, start, endExclusive)
	if err != nil {
		return Statistics{}, fmt.Errorf("aggregate report income sources: %w", err)
	}
	defer incomeSourceRows.Close()
	for incomeSourceRows.Next() {
		var value MerchantTotal
		if err := incomeSourceRows.Scan(&value.Merchant, &value.Amount, &value.TransactionCount); err != nil {
			return Statistics{}, fmt.Errorf("scan report income source: %w", err)
		}
		statistics.TopIncomeSources = append(statistics.TopIncomeSources, value)
	}
	if err := incomeSourceRows.Err(); err != nil {
		return Statistics{}, fmt.Errorf("iterate report income sources: %w", err)
	}
	incomeSourceRows.Close()

	return statistics, nil
}

func (r *Repository) CreateAIJob(ctx context.Context, job AIJob) (AIJob, bool, error) {
	var stored AIJob
	err := r.db.QueryRow(ctx, `
		INSERT INTO ai_report_jobs (id, input_hash, snapshot_hash, range_start, range_end, comparison_mode, comparison_start, comparison_end, snapshot, prompt, model, status, max_attempts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, 'queued', $12)
		ON CONFLICT (input_hash) DO UPDATE SET
			status = CASE WHEN ai_report_jobs.status = 'failed' THEN 'queued' ELSE ai_report_jobs.status END,
			attempts = CASE WHEN ai_report_jobs.status = 'failed' THEN 0 ELSE ai_report_jobs.attempts END,
			next_attempt_at = CASE WHEN ai_report_jobs.status = 'failed' THEN NOW() ELSE ai_report_jobs.next_attempt_at END,
			error_code = CASE WHEN ai_report_jobs.status = 'failed' THEN NULL ELSE ai_report_jobs.error_code END,
			error_message = CASE WHEN ai_report_jobs.status = 'failed' THEN NULL ELSE ai_report_jobs.error_message END,
			updated_at = NOW()
		RETURNING id::text, status, snapshot_hash, model, content, error_code, created_at, completed_at, prompt, attempts, max_attempts, (xmax = 0)
	`, job.ID, job.InputHash, job.SnapshotHash, job.RangeStart, job.RangeEnd, job.ComparisonMode, job.ComparisonStart, job.ComparisonEnd, job.Snapshot, job.Prompt, job.Model, job.MaxAttempts).Scan(
		&stored.ID, &stored.Status, &stored.SnapshotHash, &stored.Model, &stored.Content, &stored.ErrorCode, &stored.CreatedAt, &stored.CompletedAt, &stored.Prompt, &stored.Attempts, &stored.MaxAttempts, &stored.Created,
	)
	if err != nil {
		return AIJob{}, false, fmt.Errorf("create AI report job: %w", err)
	}
	return stored, !stored.Created, nil
}

func (r *Repository) FindAIJob(ctx context.Context, id string) (AIJob, bool, error) {
	var job AIJob
	err := r.db.QueryRow(ctx, `
		SELECT id::text, status, snapshot_hash, model, content, error_code, created_at, completed_at, attempts, max_attempts
		FROM ai_report_jobs WHERE id = $1
	`, id).Scan(&job.ID, &job.Status, &job.SnapshotHash, &job.Model, &job.Content, &job.ErrorCode, &job.CreatedAt, &job.CompletedAt, &job.Attempts, &job.MaxAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return AIJob{}, false, nil
	}
	if err != nil {
		return AIJob{}, false, fmt.Errorf("find AI report job: %w", err)
	}
	return job, true, nil
}

func (r *Repository) ClaimAIJob(ctx context.Context) (AIJob, bool, error) {
	var job AIJob
	err := r.db.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id FROM ai_report_jobs
			WHERE (status = 'queued' AND next_attempt_at <= NOW())
				OR (status = 'running' AND lease_expires_at < NOW())
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED LIMIT 1
		)
		UPDATE ai_report_jobs j
		SET status = 'running', attempts = attempts + 1, started_at = COALESCE(started_at, NOW()),
			lease_expires_at = NOW() + INTERVAL '60 seconds', updated_at = NOW(), error_code = NULL, error_message = NULL
		FROM candidate WHERE j.id = candidate.id
		RETURNING j.id::text, j.status, j.snapshot_hash, j.model, j.prompt, j.attempts, j.max_attempts
	`).Scan(&job.ID, &job.Status, &job.SnapshotHash, &job.Model, &job.Prompt, &job.Attempts, &job.MaxAttempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return AIJob{}, false, nil
	}
	if err != nil {
		return AIJob{}, false, fmt.Errorf("claim AI report job: %w", err)
	}
	return job, true, nil
}

func (r *Repository) CompleteAIJob(ctx context.Context, id, content string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ai_report_jobs
		SET status = 'complete', content = $2, completed_at = NOW(), lease_expires_at = NULL, updated_at = NOW()
		WHERE id = $1 AND status = 'running'
	`, id, content)
	if err != nil {
		return fmt.Errorf("complete AI report job: %w", err)
	}
	return nil
}

func (r *Repository) RetryOrFailAIJob(ctx context.Context, job AIJob) error {
	if job.Attempts >= job.MaxAttempts {
		_, err := r.db.Exec(ctx, `
			UPDATE ai_report_jobs SET status = 'failed', error_code = 'provider_unavailable', error_message = 'AI provider is temporarily unavailable', lease_expires_at = NULL, completed_at = NOW(), updated_at = NOW()
			WHERE id = $1 AND status = 'running'
		`, job.ID)
		if err != nil {
			return fmt.Errorf("fail AI report job: %w", err)
		}
		return nil
	}
	delay := time.Duration(1<<uint(job.Attempts-1)) * time.Second
	_, err := r.db.Exec(ctx, `
		UPDATE ai_report_jobs
		SET status = 'queued', next_attempt_at = NOW() + $2::interval, lease_expires_at = NULL,
			error_code = 'provider_unavailable', error_message = 'AI provider is temporarily unavailable', updated_at = NOW()
		WHERE id = $1 AND status = 'running'
	`, job.ID, fmt.Sprintf("%f seconds", delay.Seconds()))
	if err != nil {
		return fmt.Errorf("retry AI report job: %w", err)
	}
	return nil
}
