package account

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidName = errors.New("account name is required")
	ErrInvalidType = errors.New("invalid account type")
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{
		repository: repository,
	}
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
