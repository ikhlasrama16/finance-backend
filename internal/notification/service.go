package notification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"finance-monitor/backend/internal/account"
	"finance-monitor/backend/internal/category"
	"finance-monitor/backend/internal/rule"
	"finance-monitor/backend/internal/transaction"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSourceAppRequired = errors.New("source_app is required")
	ErrBodyRequired      = errors.New("body is required")
	ErrInvalidReceivedAt = errors.New("invalid received_at")
	ErrDuplicate         = errors.New("notification already exists")
)

type Service struct {
	repository            *Repository
	db                    *pgxpool.Pool
	accountRepository     accountResolver
	categoryRepository    categoryResolver
	transactionRepository parsedTransactionRepository
	ruleRepository        ruleRepository
}

type accountResolver interface {
	GetByName(context.Context, string) (account.Account, error)
}
type categoryResolver interface {
	GetByNameAndType(context.Context, string, string) (category.Category, error)
	GetByID(context.Context, int64) (category.Category, error)
}
type parsedTransactionRepository interface {
	CreateParsed(context.Context, pgx.Tx, transaction.CreateParsedInput) (transaction.Transaction, error)
}
type ruleRepository interface {
	ListActiveParserRules(context.Context) ([]rule.ParserRule, error)
	ListActiveCategoryRules(context.Context) ([]rule.CategoryRule, error)
}

func NewService(repository *Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func NewProcessingService(db *pgxpool.Pool, repository *Repository, accountRepository *account.Repository, categoryRepository *category.Repository, transactionRepository *transaction.Repository, ruleRepositories ...*rule.Repository) *Service {
	var ruleRepository *rule.Repository
	if len(ruleRepositories) > 0 {
		ruleRepository = ruleRepositories[0]
	}
	return &Service{repository: repository, db: db, accountRepository: accountRepository, categoryRepository: categoryRepository, transactionRepository: transactionRepository, ruleRepository: ruleRepository}
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Notification, error) {
	input.SourceApp = strings.TrimSpace(input.SourceApp)
	input.Body = strings.TrimSpace(input.Body)

	if input.SourceApp == "" {
		return Notification{}, ErrSourceAppRequired
	}

	if input.Body == "" {
		return Notification{}, ErrBodyRequired
	}

	receivedAt, err := time.Parse(
		time.RFC3339,
		input.ReceivedAt,
	)
	if err != nil {
		return Notification{}, ErrInvalidReceivedAt
	}

	fingerprint := generateFingerprint(
		input.SourceApp,
		input.Title,
		input.Body,
		receivedAt,
	)

	notification := Notification{
		SourceApp:   input.SourceApp,
		Title:       input.Title,
		Body:        input.Body,
		ReceivedAt:  receivedAt,
		Status:      "pending",
		RawPayload:  input.RawPayload,
		Fingerprint: &fingerprint,
	}

	created, err := s.repository.Create(
		ctx,
		notification,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" {
			return Notification{}, ErrDuplicate
		}

		return Notification{}, err
	}

	return created, nil
}

func (s *Service) Ingest(ctx context.Context, input CreateInput) (IngestionResult, error) {
	raw, err := s.Create(ctx, input)
	if err != nil {
		return IngestionResult{}, err
	}
	return s.process(ctx, raw)
}

func generateFingerprint(
	sourceApp string,
	title *string,
	body string,
	receivedAt time.Time,
) string {
	titleValue := ""

	if title != nil {
		titleValue = *title
	}

	value := strings.Join(
		[]string{
			sourceApp,
			titleValue,
			body,
			receivedAt.UTC().Format(time.RFC3339),
		},
		"|",
	)

	hash := sha256.Sum256([]byte(value))

	return hex.EncodeToString(hash[:])
}
