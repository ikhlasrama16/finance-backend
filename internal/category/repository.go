package category

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func (r *Repository) GetByNameAndType(ctx context.Context, name, categoryType string) (Category, error) {
	var category Category
	err := r.db.QueryRow(ctx, `
		SELECT id, name, type, created_at, updated_at
		FROM categories
		WHERE LOWER(name) = LOWER($1) AND type = $2
	`, strings.TrimSpace(name), categoryType).Scan(
		&category.ID, &category.Name, &category.Type,
		&category.CreatedAt, &category.UpdatedAt,
	)
	if err != nil {
		return Category{}, fmt.Errorf("get category by name and type: %w", err)
	}
	return category, nil
}

func (r *Repository) ListByType(ctx context.Context, categoryType string) ([]Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, type, created_at, updated_at
		FROM categories
		WHERE type = $1
		ORDER BY name
	`, categoryType)
	if err != nil {
		return nil, fmt.Errorf("query categories by type: %w", err)
	}
	defer rows.Close()

	categories := make([]Category, 0)
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.ID, &category.Name, &category.Type, &category.CreatedAt, &category.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories by type: %w", err)
	}
	return categories, nil
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]Category, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			id,
			name,
			type,
			created_at,
			updated_at
		FROM categories
		ORDER BY type, name
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	categories := make([]Category, 0)

	for rows.Next() {
		var category Category

		if err := rows.Scan(
			&category.ID,
			&category.Name,
			&category.Type,
			&category.CreatedAt,
			&category.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", err)
	}

	return categories, nil
}

func (r *Repository) Create(
	ctx context.Context,
	input CreateInput,
) (Category, error) {
	var category Category

	err := r.db.QueryRow(
		ctx,
		`
		INSERT INTO categories (
			name,
			type
		)
		VALUES ($1, $2)
		RETURNING
			id,
			name,
			type,
			created_at,
			updated_at
		`,
		input.Name,
		input.Type,
	).Scan(
		&category.ID,
		&category.Name,
		&category.Type,
		&category.CreatedAt,
		&category.UpdatedAt,
	)

	if err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	}

	return category, nil
}

func (r *Repository) GetByID(
	ctx context.Context,
	id int64,
) (Category, error) {
	var category Category

	err := r.db.QueryRow(
		ctx,
		`
		SELECT
			id,
			name,
			type,
			created_at,
			updated_at
		FROM categories
		WHERE id = $1
		`,
		id,
	).Scan(
		&category.ID,
		&category.Name,
		&category.Type,
		&category.CreatedAt,
		&category.UpdatedAt,
	)

	if err != nil {
		return Category{}, fmt.Errorf("get category: %w", err)
	}

	return category, nil
}
