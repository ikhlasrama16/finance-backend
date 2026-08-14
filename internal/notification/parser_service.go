package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"finance-monitor/backend/internal/category"
	"finance-monitor/backend/internal/parser"
	"finance-monitor/backend/internal/rule"
	"finance-monitor/backend/internal/transaction"
)

func (s *Service) process(ctx context.Context, raw Notification) (IngestionResult, error) {
	input := parser.Input{SourceApp: raw.SourceApp, Text: raw.Body}
	if raw.Title != nil {
		input.Title = *raw.Title
	}
	parsed, parserName, err := s.parse(ctx, input)
	if err != nil {
		return s.failed(ctx, raw, "parser error", parserName)
	}
	if parsed == nil {
		return s.failed(ctx, raw, "notification format not recognized", parserName)
	}
	if parsed.Ignore {
		if err := s.repository.MarkIgnored(ctx, raw.ID, parserName); err != nil {
			return IngestionResult{}, err
		}
		return IngestionResult{RawNotificationID: raw.ID, Status: "ignored"}, nil
	}
	if err := s.applyCategoryRule(ctx, parsed); err != nil {
		return s.failed(ctx, raw, "category rule evaluation failed", parserName)
	}

	parsedInput, err := s.resolve(ctx, raw, parsed)
	if err != nil {
		return s.failed(ctx, raw, err.Error(), parserName)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return IngestionResult{}, fmt.Errorf("begin notification transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	created, err := s.transactionRepository.CreateParsed(ctx, tx, parsedInput)
	if err != nil {
		return IngestionResult{}, err
	}
	if err := s.repository.MarkParsed(ctx, tx, raw.ID, created.ID, parserName); err != nil {
		return IngestionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return IngestionResult{}, fmt.Errorf("commit notification transaction: %w", err)
	}
	return IngestionResult{RawNotificationID: raw.ID, Status: "parsed", TransactionID: &created.ID}, nil
}

func (s *Service) failed(ctx context.Context, raw Notification, reason, parserName string) (IngestionResult, error) {
	if err := s.repository.MarkFailed(ctx, raw.ID, reason, parserName); err != nil {
		return IngestionResult{}, err
	}
	return IngestionResult{RawNotificationID: raw.ID, Status: "failed"}, nil
}

func (s *Service) parse(ctx context.Context, input parser.Input) (*parser.Result, string, error) {
	if s.ruleRepository == nil {
		result, err := parser.Parse(input)
		return result, "parser", err
	}
	rules, err := s.ruleRepository.ListActiveParserRules(ctx)
	if err != nil {
		return nil, "rule", err
	}
	result, err := parser.NewRegistry().ParseWithRules(input, func(value parser.Input) (*parser.Result, bool, error) {
		matched := rule.FirstParserRule(rules, value.SourceApp, strings.Join([]string{value.Title, value.Text}, " "))
		if matched == nil {
			return nil, false, nil
		}
		if strings.EqualFold(matched.Action, "IGNORE") {
			return &parser.Result{Ignore: true, ParseStatus: "IGNORED", Confidence: matched.Confidence}, true, nil
		}
		if !strings.EqualFold(matched.Action, "PARSE") || matched.TransactionType == nil {
			return nil, false, nil
		}
		amount, ok := parser.ExtractBestAmount(strings.Join([]string{value.Title, value.Text}, " "))
		if !ok {
			return nil, false, nil
		}
		typ := strings.ToLower(strings.TrimSpace(*matched.TransactionType))
		if typ != "income" && typ != "expense" && typ != "transfer" {
			return nil, false, nil
		}
		parsedResult := &parser.Result{Type: typ, Amount: amount, ParseStatus: "RULE", Confidence: matched.Confidence, CategoryID: matched.CategoryID}
		if matched.Merchant != nil {
			parsedResult.Merchant = *matched.Merchant
		}
		return parsedResult, true, nil
	})
	if err != nil {
		return nil, "rule", err
	}
	if result != nil && (result.ParseStatus == "RULE" || result.ParseStatus == "IGNORED") {
		return result, "rule", nil
	}
	return result, "parser", nil
}

func (s *Service) applyCategoryRule(ctx context.Context, result *parser.Result) error {
	if result.Type != "expense" || s.ruleRepository == nil {
		return nil
	}
	rules, err := s.ruleRepository.ListActiveCategoryRules(ctx)
	if err != nil {
		return err
	}
	haystack := strings.Join([]string{result.Merchant, result.Description, result.SourceAccountName}, " ")
	if matched := rule.FirstCategoryRule(rules, haystack); matched != nil {
		result.CategoryID = &matched.CategoryID
		result.CategoryName = ""
	}
	return nil
}

func (s *Service) resolve(ctx context.Context, raw Notification, result *parser.Result) (transaction.CreateParsedInput, error) {
	if result.Amount <= 0 {
		return transaction.CreateParsedInput{}, errors.New("parsed amount must be greater than zero")
	}
	input := transaction.CreateParsedInput{Type: result.Type, Amount: result.Amount, OccurredAt: raw.ReceivedAt.Format("2006-01-02T15:04:05Z07:00"), RawNotificationID: raw.ID, ParseStatus: result.ParseStatus, Confidence: result.Confidence}
	if input.ParseStatus == "" {
		input.ParseStatus = "NEEDS_REVIEW"
	}
	if strings.TrimSpace(result.Description) != "" {
		input.Description = stringPointer(result.Description)
	}
	if strings.TrimSpace(result.Merchant) != "" {
		input.Merchant = stringPointer(result.Merchant)
	}

	if result.Type == "income" || result.Type == "expense" {
		if result.Type == "income" {
			input.DestinationAccountID = accountID(ctx, s.accountRepository, result.DestinationAccountName)
		} else {
			input.SourceAccountID = accountID(ctx, s.accountRepository, result.SourceAccountName)
		}
		if (result.Type == "income" && input.DestinationAccountID == nil) || (result.Type == "expense" && input.SourceAccountID == nil) {
			return transaction.CreateParsedInput{}, errors.New("required account not found")
		}
		var cat category.Category
		var err error
		if result.CategoryID != nil {
			cat, err = s.categoryRepository.GetByID(ctx, *result.CategoryID)
		} else {
			cat, err = s.categoryRepository.GetByNameAndType(ctx, result.CategoryName, result.Type)
		}
		if err != nil {
			return transaction.CreateParsedInput{}, errors.New("required category not found")
		}
		if cat.Type != result.Type {
			return transaction.CreateParsedInput{}, errors.New("category type does not match transaction type")
		}
		input.CategoryID = &cat.ID
	} else if result.Type == "transfer" {
		input.SourceAccountID = accountID(ctx, s.accountRepository, result.SourceAccountName)
		input.DestinationAccountID = accountID(ctx, s.accountRepository, result.DestinationAccountName)
		if input.SourceAccountID == nil || input.DestinationAccountID == nil {
			return transaction.CreateParsedInput{}, errors.New("required account not found")
		}
		if *input.SourceAccountID == *input.DestinationAccountID {
			return transaction.CreateParsedInput{}, errors.New("source and destination account cannot be the same")
		}
	} else {
		return transaction.CreateParsedInput{}, errors.New("invalid parsed transaction type")
	}
	return input, nil
}

func accountID(ctx context.Context, repository accountResolver, name string) *int64 {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	value, err := repository.GetByName(ctx, name)
	if err != nil {
		return nil
	}
	return &value.ID
}
func stringPointer(value string) *string { return &value }
