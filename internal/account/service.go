package account

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidName               = errors.New("account name is required")
	ErrInvalidType               = errors.New("invalid account type")
	ErrInvalidAccountID          = errors.New("invalid account id")
	ErrInvalidReconciliationNote = errors.New("reconciliation note is too long")
	ErrAccountNotFound           = errors.New("account not found")
)

type repository interface {
	List(context.Context) ([]Account, error)
	Create(context.Context, CreateInput) (Account, error)
	Reconcile(context.Context, ReconcileInput) (Reconciliation, error)
}

type Service struct {
	repository repository
}

func NewService(repository repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) Reconcile(ctx context.Context, accountID, actualBalance int64, note string) (Reconciliation, error) {
	if accountID <= 0 {
		return Reconciliation{}, ErrInvalidAccountID
	}
	note = strings.TrimSpace(note)
	if len(note) > 500 {
		return Reconciliation{}, ErrInvalidReconciliationNote
	}
	description := "Balance reconciliation"
	if note != "" {
		description += " - " + note
	}
	return s.repository.Reconcile(ctx, ReconcileInput{
		AccountID: accountID, ActualBalance: actualBalance, Description: description,
	})
}

func (s *Service) List(ctx context.Context) ([]Account, error) {
	return s.repository.List(ctx)
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Account, error) {
	input.Name = strings.TrimSpace(input.Name)

	if input.Name == "" {
		return Account{}, ErrInvalidName
	}

	switch input.Type {
	case "bank", "ewallet", "cash", "other":
	default:
		return Account{}, ErrInvalidType
	}

	return s.repository.Create(ctx, input)
}
