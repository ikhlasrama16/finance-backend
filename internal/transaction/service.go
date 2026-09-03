package transaction

import (
	"context"
	"errors"
	"strings"
	"time"

	"finance-monitor/backend/internal/category"
	"finance-monitor/backend/internal/rule"

	"github.com/jackc/pgx/v5"
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
	ErrTransactionNotFound  = errors.New("transaction not found")
	ErrCategoryNotFound     = errors.New("category not found")
	ErrNoUpdateFields       = errors.New("at least one editable field is required")
	ErrReconciliationDelete = errors.New("reconciliation transactions cannot be deleted directly")
)

type repository interface {
	List(context.Context) ([]Transaction, error)
	Create(context.Context, Transaction) (Transaction, error)
	GetByID(context.Context, int64) (Transaction, error)
	Update(context.Context, Transaction) (Transaction, error)
	Delete(context.Context, int64) error
}

type categoryRepository interface {
	GetByID(context.Context, int64) (category.Category, error)
}

type ruleRepository interface {
	CreateCategoryRule(context.Context, string, int64, float64, int) (rule.CategoryRule, error)
}

type Service struct {
	repository         repository
	categoryRepository categoryRepository
	ruleRepository     ruleRepository
}

func NewService(repository repository, categoryRepository categoryRepository) *Service {
	return &Service{repository: repository, categoryRepository: categoryRepository}
}

func (s *Service) WithRuleRepository(r ruleRepository) *Service {
	s.ruleRepository = r
	return s
}

func (s *Service) List(ctx context.Context) ([]Transaction, error) {
	return s.repository.List(ctx)
}

func (s *Service) GetByID(ctx context.Context, id int64) (Transaction, error) {
	return s.getTransaction(ctx, id)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Transaction, error) {
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
		cat, err := s.getCategory(ctx, *input.CategoryID)
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
		cat, err := s.getCategory(ctx, *input.CategoryID)
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
	return s.repository.Create(ctx, Transaction{
		Type: input.Type, Amount: input.Amount, SourceAccountID: input.SourceAccountID,
		DestinationAccountID: input.DestinationAccountID, CategoryID: input.CategoryID,
		Description: normalizedOptionalString(input.Description), OccurredAt: occurredAt,
	})
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (Transaction, error) {
	if id <= 0 {
		return Transaction{}, ErrTransactionNotFound
	}
	if input.CategoryID == nil && input.Merchant == nil && input.Description == nil {
		return Transaction{}, ErrNoUpdateFields
	}
	transaction, err := s.getTransaction(ctx, id)
	if err != nil {
		return Transaction{}, err
	}
	if input.CategoryID != nil {
		if transaction.Type == "transfer" {
			return Transaction{}, ErrCategoryNotAllowed
		}
		cat, err := s.getCategory(ctx, *input.CategoryID)
		if err != nil {
			return Transaction{}, err
		}
		if cat.Type != transaction.Type {
			return Transaction{}, ErrCategoryTypeMismatch
		}
		transaction.CategoryID = &cat.ID
	}
	if input.Merchant != nil {
		transaction.Merchant = normalizedOptionalString(input.Merchant)
	}
	if input.Description != nil {
		transaction.Description = normalizedOptionalString(input.Description)
	}
	updated, err := s.repository.Update(ctx, transaction)
	if err != nil {
		return Transaction{}, err
	}
	if input.LearnRule != nil && *input.LearnRule && s.ruleRepository != nil && updated.Merchant != nil && updated.CategoryID != nil {
		merchant := strings.TrimSpace(*updated.Merchant)
		if merchant != "" {
			_, _ = s.ruleRepository.CreateCategoryRule(ctx, merchant, *updated.CategoryID, 1.0, 10)
		}
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int64) (DeleteResult, error) {
	if id <= 0 {
		return DeleteResult{}, ErrTransactionNotFound
	}
	transaction, err := s.getTransaction(ctx, id)
	if err != nil {
		return DeleteResult{}, err
	}
	if transaction.Source == "reconcile" {
		return DeleteResult{}, ErrReconciliationDelete
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DeleteResult{}, ErrTransactionNotFound
		}
		return DeleteResult{}, err
	}
	return DeleteResult{ID: id}, nil
}

func (s *Service) getTransaction(ctx context.Context, id int64) (Transaction, error) {
	transaction, err := s.repository.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transaction{}, ErrTransactionNotFound
	}
	return transaction, err
}

func (s *Service) getCategory(ctx context.Context, id int64) (category.Category, error) {
	value, err := s.categoryRepository.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return category.Category{}, ErrCategoryNotFound
	}
	return value, err
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
