package category

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidName = errors.New("category name is required")
	ErrInvalidType = errors.New("invalid category type")
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context) ([]Category, error) {
	return s.repository.List(ctx)
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Category, error) {
	input.Name = strings.TrimSpace(input.Name)

	if input.Name == "" {
		return Category{}, ErrInvalidName
	}

	switch input.Type {
	case "income", "expense":
	default:
		return Category{}, ErrInvalidType
	}

	return s.repository.Create(ctx, input)
}

func (s *Service) GetByID(
	ctx context.Context,
	id int64,
) (Category, error) {
	return s.repository.GetByID(ctx, id)
}
