package rule

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) ListActiveParserRules(ctx context.Context) ([]ParserRule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, source_app, keyword, action, transaction_type, category_id, merchant, confidence, priority
		FROM parser_rules WHERE is_active = TRUE ORDER BY priority ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active parser rules: %w", err)
	}
	defer rows.Close()
	rules := make([]ParserRule, 0)
	for rows.Next() {
		var value ParserRule
		if err := rows.Scan(&value.ID, &value.SourceApp, &value.Keyword, &value.Action, &value.TransactionType, &value.CategoryID, &value.Merchant, &value.Confidence, &value.Priority); err != nil {
			return nil, fmt.Errorf("scan parser rule: %w", err)
		}
		rules = append(rules, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate parser rules: %w", err)
	}
	return rules, nil
}

func (r *Repository) ListActiveCategoryRules(ctx context.Context) ([]CategoryRule, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, keyword, category_id, confidence, priority
		FROM category_rules WHERE is_active = TRUE ORDER BY priority ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active category rules: %w", err)
	}
	defer rows.Close()
	rules := make([]CategoryRule, 0)
	for rows.Next() {
		var value CategoryRule
		if err := rows.Scan(&value.ID, &value.Keyword, &value.CategoryID, &value.Confidence, &value.Priority); err != nil {
			return nil, fmt.Errorf("scan category rule: %w", err)
		}
		rules = append(rules, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category rules: %w", err)
	}
	return rules, nil
}
