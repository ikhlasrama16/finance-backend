package transaction

import (
	"context"
	"errors"
	"strings"
	"time"

	"finance-monitor/backend/internal/category"
)

var (
	ErrInvalidType          = errors.New("invalid transaction type")
	ErrInvalidAmount        = errors.New("amount must be greater than zero")
	ErrSourceRequired       = errors.New("source account is required")
	ErrDestinationRequired  = errors.New("destination account is required")
	ErrSameAccount          = errors.New("source and destination account cannot be the same")
	ErrInvalidOccurredAt    = errors.New("invalid occurred_at")
	ErrCategoryRequired     = errors.New("category is required")
	ErrCategoryNotAllowed   = errors.New("category is not allowed for transfer")
	ErrCategoryTypeMismatch = errors.New("category type does not match transaction type")
)

type Service struct {
	repository         *Repository
	categoryRepository *category.Repository
}

func NewService(
	repository *Repository,
	categoryRepository *category.Repository,
) *Service {
	return &Service{
		repository:         repository,
		categoryRepository: categoryRepository,
	}
}

func (s *Service) List(ctx context.Context) ([]Transaction, error) {
	return s.repository.List(ctx)
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Transaction, error) {
	if input.Amount <= 0 {
		return Transaction{}, ErrInvalidAmount
	}

	switch input.Type {
	case "income":
		if input.DestinationAccountID == nil {
			return Transaction{}, ErrDestinationRequired
		}

		if input.CategoryID == nil {
			return Transaction{}, ErrCategoryRequired
		}

		cat, err := s.categoryRepository.GetByID(
			ctx,
			*input.CategoryID,
		)
		if err != nil {
			return Transaction{}, err
		}

		if cat.Type != "income" {
			return Transaction{}, ErrCategoryTypeMismatch
		}

	case "expense":
		if input.SourceAccountID == nil {
			return Transaction{}, ErrSourceRequired
		}

		if input.CategoryID == nil {
			return Transaction{}, ErrCategoryRequired
		}

		cat, err := s.categoryRepository.GetByID(
			ctx,
			*input.CategoryID,
		)
		if err != nil {
			return Transaction{}, err
		}

		if cat.Type != "expense" {
			return Transaction{}, ErrCategoryTypeMismatch
		}

	case "transfer":
		if input.SourceAccountID == nil {
			return Transaction{}, ErrSourceRequired
		}

		if input.DestinationAccountID == nil {
			return Transaction{}, ErrDestinationRequired
		}

		if *input.SourceAccountID == *input.DestinationAccountID {
			return Transaction{}, ErrSameAccount
		}

		if input.CategoryID != nil {
			return Transaction{}, ErrCategoryNotAllowed
		}

	default:
		return Transaction{}, ErrInvalidType
	}

	occurredAt, err := time.Parse(time.RFC3339, input.OccurredAt)
	if err != nil {
		return Transaction{}, ErrInvalidOccurredAt
	}

	var description *string

	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)

		if value != "" {
			description = &value
		}
	}

	tx := Transaction{
		Type:                 input.Type,
		Amount:               input.Amount,
		SourceAccountID:      input.SourceAccountID,
		DestinationAccountID: input.DestinationAccountID,
		CategoryID:           input.CategoryID,
		Description:          description,
		OccurredAt:           occurredAt,
	}

	return s.repository.Create(ctx, tx)
}
