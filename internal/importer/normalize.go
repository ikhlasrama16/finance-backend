package importer

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

var jakarta = time.FixedZone("WIB", 7*60*60)

func ReadCSV(r io.Reader) ([]LegacyRow, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	indexes := make(map[string]int, len(header))
	for i, value := range header {
		indexes[strings.TrimSpace(strings.ToLower(value))] = i
	}
	required := []string{"id", "timestamp", "account", "type", "amount", "merchant", "category", "description", "source", "parse_status", "confidence", "is_synthetic", "transfer_group_id"}
	for _, name := range required {
		if _, ok := indexes[name]; !ok {
			return nil, fmt.Errorf("CSV column %q is required", name)
		}
	}
	var rows []LegacyRow
	for line := 2; ; line++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row %d: %w", line, err)
		}
		value := func(name string) string {
			index := indexes[name]
			if index >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[index])
		}
		rows = append(rows, LegacyRow{ID: value("id"), Timestamp: value("timestamp"), Account: value("account"), Type: value("type"), Amount: value("amount"), Merchant: value("merchant"), Category: value("category"), Description: value("description"), Source: value("source"), ParseStatus: value("parse_status"), Confidence: value("confidence"), IsSynthetic: value("is_synthetic"), TransferGroupID: value("transfer_group_id")})
	}
	return rows, nil
}

func Normalize(rows []LegacyRow, accounts []Account, categories []Category) ResolvedData {
	accountMap := make(map[string]Account, len(accounts))
	for _, account := range accounts {
		if forbiddenAccount(account.Name) {
			continue
		}
		accountMap[normalizeName(account.Name)] = account
	}
	categoryMap := make(map[string]Category, len(categories))
	for _, category := range categories {
		categoryMap[categoryKey(category.Name, category.Type)] = category
	}

	result := ResolvedData{Summary: Summary{RowsRead: len(rows)}}
	groups := make(map[string][]LegacyRow)
	var standalone []LegacyRow
	for _, row := range rows {
		if strings.TrimSpace(row.TransferGroupID) == "" {
			standalone = append(standalone, row)
		} else {
			groups[row.TransferGroupID] = append(groups[row.TransferGroupID], row)
		}
	}

	for _, row := range standalone {
		if isSynthetic(row) {
			result.Summary.SyntheticRowsIgnored++
			continue
		}
		var tx Transaction
		var reason, kind, action string
		if strings.EqualFold(strings.TrimSpace(row.Type), "TRANSFER_IN") || strings.EqualFold(strings.TrimSpace(row.Type), "TRANSFER_OUT") {
			tx, reason, action = normalizeSingleTransfer(row, accountMap, categoryMap)
		} else {
			tx, reason, kind = normalizeRow(row, accountMap, categoryMap)
		}
		if kind == "unknown" {
			result.Summary.UnknownRowsSkipped++
		} else if action == "skip" {
			result.Summary.MarketplaceSupportSkipped++
		} else if reason != "" {
			result.Summary.UnresolvedRows = append(result.Summary.UnresolvedRows, unresolvedRow(row, reason))
		} else {
			result.Transactions = append(result.Transactions, tx)
			countImported(&result.Summary, tx.Type)
			if action == "external-income" {
				result.Summary.ExternalTransferInsConverted++
			}
			if action == "external-expense" {
				result.Summary.ExternalTransferOutsConverted++
			}
		}
	}

	for groupID, grouped := range groups {
		active := make([]LegacyRow, 0, len(grouped))
		for _, row := range grouped {
			if isSynthetic(row) {
				result.Summary.SyntheticRowsIgnored++
			}
			active = append(active, row)
		}
		if len(active) == 0 {
			continue
		}
		tx, reason, action := normalizeTransferGroup(groupID, active, accountMap, categoryMap)
		if action == "skip" {
			result.Summary.MarketplaceSupportSkipped++
			continue
		}
		if reason != "" {
			result.Summary.UnresolvedRows = append(result.Summary.UnresolvedRows, unresolvedGroup(groupID, active, reason))
			continue
		}
		result.Transactions = append(result.Transactions, tx)
		if tx.Type == "transfer" {
			result.Summary.TransfersCollapsed++
		} else if tx.Type == "income" {
			result.Summary.IncomeRowsImported++
			result.Summary.ExternalTransferInsConverted++
		} else if tx.Type == "expense" {
			result.Summary.ExpenseRowsImported++
			result.Summary.ExternalTransferOutsConverted++
		}
	}
	return result
}

func normalizeSingleTransfer(row LegacyRow, accounts map[string]Account, categories map[string]Category) (Transaction, string, string) {
	if isMarketplaceSupport(row) {
		return Transaction{}, "", "skip"
	}
	amount, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(row.Amount), ".", ""), 10, 64)
	if err != nil || amount <= 0 {
		return Transaction{}, "invalid amount", ""
	}
	when, err := ParseTimestamp(row.Timestamp)
	if err != nil {
		return Transaction{}, err.Error(), ""
	}
	account, ok := resolveAccount(row.Account, accounts)
	if !ok {
		return Transaction{}, "account not found: " + row.Account, ""
	}
	counterparty := findOwnedAccount(row.Merchant+" "+row.Description, accounts, account.ID)
	tx := Transaction{LegacyID: "transaction:" + row.ID, Amount: amount, OccurredAt: when, ParseStatus: normalizeParseStatus(row.ParseStatus), Merchant: optionalString(row.Merchant), Description: optionalString(row.Description), Confidence: parseConfidence(row.Confidence)}
	if strings.EqualFold(strings.TrimSpace(row.Type), "TRANSFER_OUT") {
		tx.SourceAccountID = intPointer(account.ID)
		if counterparty != nil {
			tx.Type, tx.DestinationAccount = "transfer", intPointer(counterparty.ID)
			return tx, "", ""
		}
		if isAmbiguousCounterparty(row.Merchant) || strings.TrimSpace(row.Merchant) == "" {
			return Transaction{}, counterpartyReason(row, "destination"), ""
		}
		category, ok := categories[categoryKey("Belum Dikategorikan", "expense")]
		if !ok {
			return Transaction{}, "expense category not found: Belum Dikategorikan", ""
		}
		tx.Type, tx.CategoryID = "expense", intPointer(category.ID)
		return tx, "", "external-expense"
	} else {
		tx.DestinationAccount = intPointer(account.ID)
		if counterparty != nil {
			tx.Type, tx.SourceAccountID = "transfer", intPointer(counterparty.ID)
			return tx, "", ""
		}
		if isShopeePayTopUp(row) {
			return Transaction{}, "source account unknown for ShopeePay top-up", ""
		}
		if isAmbiguousCounterparty(row.Merchant) || strings.TrimSpace(row.Merchant) == "" {
			return Transaction{}, counterpartyReason(row, "source"), ""
		}
		category, ok := categories[categoryKey("Pemasukan", "income")]
		if !ok {
			return Transaction{}, "income category not found: Pemasukan", ""
		}
		tx.Type, tx.CategoryID = "income", intPointer(category.ID)
		return tx, "", "external-income"
	}
}

func normalizeRow(row LegacyRow, accounts map[string]Account, categories map[string]Category) (Transaction, string, string) {
	typ := strings.ToUpper(strings.TrimSpace(row.Type))
	if typ == "UNKNOWN" {
		return Transaction{}, "", "unknown"
	}
	amount, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(row.Amount), ".", ""), 10, 64)
	if err != nil || amount <= 0 {
		return Transaction{}, "invalid amount", ""
	}
	when, err := ParseTimestamp(row.Timestamp)
	if err != nil {
		return Transaction{}, err.Error(), ""
	}
	tx := Transaction{LegacyID: "transaction:" + row.ID, Amount: amount, OccurredAt: when, ParseStatus: normalizeParseStatus(row.ParseStatus)}
	tx.Merchant = optionalString(row.Merchant)
	tx.Description = optionalString(row.Description)
	tx.Confidence = parseConfidence(row.Confidence)

	switch typ {
	case "EXPENSE":
		account, ok := resolveAccount(row.Account, accounts)
		if !ok {
			return Transaction{}, "source account not found: " + row.Account, ""
		}
		categoryName := row.Category
		if strings.TrimSpace(categoryName) == "" {
			categoryName = "Belum Dikategorikan"
		}
		category, ok := categories[categoryKey(categoryName, "expense")]
		if !ok {
			return Transaction{}, "expense category not found: " + categoryName, ""
		}
		tx.Type, tx.SourceAccountID, tx.CategoryID = "expense", intPointer(account.ID), intPointer(category.ID)
	case "INCOME":
		account, ok := resolveAccount(row.Account, accounts)
		if !ok {
			return Transaction{}, "destination account not found: " + row.Account, ""
		}
		categoryName := row.Category
		if strings.TrimSpace(categoryName) == "" {
			categoryName = "Pemasukan"
		}
		category, ok := categories[categoryKey(categoryName, "income")]
		if !ok {
			return Transaction{}, "income category not found: " + categoryName, ""
		}
		tx.Type, tx.DestinationAccount, tx.CategoryID = "income", intPointer(account.ID), intPointer(category.ID)
	case "TRANSFER_IN", "TRANSFER_OUT":
		return Transaction{}, "unmatched transfer must be processed as a group or resolved explicitly", ""
	default:
		return Transaction{}, "unsupported legacy type: " + row.Type, ""
	}
	return tx, "", ""
}

func normalizeTransferGroup(groupID string, rows []LegacyRow, accounts map[string]Account, categories map[string]Category) (Transaction, string, string) {
	if isMarketplaceSupportGroup(rows) {
		return Transaction{}, "", "skip"
	}
	var source, destination *Account
	var amount int64
	var when time.Time
	var representative LegacyRow
	var unresolvedAccounts []string
	var evidence strings.Builder
	for _, row := range rows {
		value, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(row.Amount), ".", ""), 10, 64)
		if err != nil || value <= 0 {
			return Transaction{}, "invalid transfer amount", ""
		}
		if amount != 0 && amount != value {
			return Transaction{}, "transfer pair amounts do not match", ""
		}
		amount = value
		parsedTime, err := ParseTimestamp(row.Timestamp)
		if err != nil {
			return Transaction{}, err.Error(), ""
		}
		if when.IsZero() || parsedTime.Before(when) {
			when, representative = parsedTime, row
		}
		evidence.WriteString(" ")
		evidence.WriteString(row.Merchant)
		evidence.WriteString(" ")
		evidence.WriteString(row.Description)
		account, ok := resolveAccount(row.Account, accounts)
		if !ok {
			unresolvedAccounts = append(unresolvedAccounts, row.Account)
			if inferred := findOwnedAccount(row.Merchant+" "+row.Description, accounts, 0); inferred != nil {
				if strings.EqualFold(strings.TrimSpace(row.Type), "TRANSFER_OUT") && source == nil {
					copy := *inferred
					source = &copy
				}
				if strings.EqualFold(strings.TrimSpace(row.Type), "TRANSFER_IN") && destination == nil {
					copy := *inferred
					destination = &copy
				}
			}
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(row.Type)) {
		case "TRANSFER_OUT":
			if source != nil && source.ID != account.ID {
				return Transaction{}, "multiple transfer sources", ""
			}
			copy := account
			source = &copy
		case "TRANSFER_IN":
			if destination != nil && destination.ID != account.ID {
				return Transaction{}, "multiple transfer destinations", ""
			}
			copy := account
			destination = &copy
		default:
			return Transaction{}, "unsupported transfer row type: " + row.Type, ""
		}
	}
	if source == nil {
		if inferred := findOwnedAccount(evidence.String(), accounts, accountID(destination)); inferred != nil {
			copy := *inferred
			source = &copy
		}
	}
	if destination == nil {
		if inferred := findOwnedAccount(evidence.String(), accounts, accountID(source)); inferred != nil {
			copy := *inferred
			destination = &copy
		}
	}
	if source == nil || destination == nil {
		if source == nil && destination != nil && isShopeePayTopUpGroup(rows) {
			return Transaction{}, "source account unknown for ShopeePay top-up", ""
		}
		if source == nil && destination != nil && externalIncomingGroup(rows) {
			category, ok := categories[categoryKey("Pemasukan", "income")]
			if !ok {
				return Transaction{}, "income category not found: Pemasukan", ""
			}
			merchant := externalMerchant(rows, accounts)
			if merchant == "" || isAmbiguousCounterparty(merchant) {
				return Transaction{}, "counterparty is ambiguous", ""
			}
			tx := importedTransferMetadata("transaction:"+groupID, representative, amount, when)
			tx.Type, tx.DestinationAccount, tx.CategoryID = "income", intPointer(destination.ID), intPointer(category.ID)
			return tx, "", "external-income"
		}
		if destination == nil && source != nil && externalOutgoingGroup(rows) {
			category, ok := categories[categoryKey("Belum Dikategorikan", "expense")]
			if !ok {
				return Transaction{}, "expense category not found: Belum Dikategorikan", ""
			}
			merchant := externalMerchant(rows, accounts)
			if merchant == "" || isAmbiguousCounterparty(merchant) {
				return Transaction{}, "counterparty is ambiguous", ""
			}
			tx := importedTransferMetadata("transaction:"+groupID, representative, amount, when)
			tx.Type, tx.SourceAccountID, tx.CategoryID = "expense", intPointer(source.ID), intPointer(category.ID)
			return tx, "", "external-expense"
		}
		if len(unresolvedAccounts) > 0 {
			return Transaction{}, "account not found: " + strings.Join(uniqueStrings(unresolvedAccounts), ", "), ""
		}
		return Transaction{}, "transfer source and destination could not both be resolved", ""
	}
	if source.ID == destination.ID {
		return Transaction{}, "transfer source and destination are the same account", ""
	}
	tx := importedTransferMetadata("transfer:"+groupID, representative, amount, when)
	tx.Type, tx.SourceAccountID, tx.DestinationAccount = "transfer", intPointer(source.ID), intPointer(destination.ID)
	return tx, "", ""
}

func ParseTimestamp(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation("1/2/2006 15:04:05", strings.TrimSpace(value), jakarta)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
	}
	return parsed, nil
}

func normalizeName(value string) string {
	value = strings.ToLower(strings.Join(strings.Fields(value), " "))
	if value == "shopeepay" {
		return "shopeepay"
	}
	return value
}

func categoryKey(name, typ string) string {
	return normalizeName(name) + "/" + strings.ToLower(strings.TrimSpace(typ))
}
func forbiddenAccount(name string) bool {
	value := normalizeName(name)
	return value == "shopee" || value == "tokopedia"
}
func findOwnedAccount(text string, accounts map[string]Account, excludeID int64) *Account {
	for _, canonical := range ownedAccountAliases {
		if !containsOwnedAlias(text, canonical.alias) {
			continue
		}
		if account, ok := accounts[normalizeName(canonical.name)]; ok && account.ID != excludeID {
			value := account
			return &value
		}
	}
	return nil
}

func importedTransferMetadata(legacyID string, row LegacyRow, amount int64, when time.Time) Transaction {
	return Transaction{LegacyID: legacyID, Amount: amount, OccurredAt: when, ParseStatus: normalizeParseStatus(row.ParseStatus), Merchant: optionalString(row.Merchant), Description: optionalString(row.Description), Confidence: parseConfidence(row.Confidence)}
}

func isAmbiguousCounterparty(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "rek")
}

func counterpartyReason(row LegacyRow, endpoint string) string {
	if endpoint == "source" && isShopeePayTopUp(row) {
		return "source account unknown for ShopeePay top-up"
	}
	return "counterparty is ambiguous"
}

func isShopeePayTopUp(row LegacyRow) bool {
	return normalizeName(row.Account) == "shopeepay" && containsPhrase(row.Description, "isi saldo shopeepay")
}

func isMarketplaceSupport(row LegacyRow) bool {
	if normalizeName(row.Account) != "shopee" {
		return false
	}
	text := row.Merchant + " " + row.Description
	return (containsPhrase(row.Description, "isi saldo berhasil") || containsPhrase(row.Description, "top up completed") || containsPhrase(row.Description, "top-up completed")) && containsOwnedAlias(text, "shopeepay")
}

func isMarketplaceSupportGroup(rows []LegacyRow) bool {
	support := false
	for _, row := range rows {
		if isMarketplaceSupport(row) {
			support = true
			continue
		}
		if !isSynthetic(row) && normalizeName(row.Account) != "shopee" {
			return false
		}
	}
	return support
}

func isShopeePayTopUpGroup(rows []LegacyRow) bool {
	for _, row := range rows {
		if isShopeePayTopUp(row) {
			return true
		}
	}
	return false
}

func externalIncomingGroup(rows []LegacyRow) bool {
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Type), "TRANSFER_IN") && strings.TrimSpace(row.Merchant) != "" && !containsAnyOwnedAccountAlias(row.Merchant) {
			return !isAmbiguousCounterparty(row.Merchant)
		}
	}
	return false
}

func externalOutgoingGroup(rows []LegacyRow) bool {
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Type), "TRANSFER_OUT") && strings.TrimSpace(row.Merchant) != "" {
			return !isAmbiguousCounterparty(row.Merchant)
		}
	}
	return false
}

func externalMerchant(rows []LegacyRow, accounts map[string]Account) string {
	for _, row := range rows {
		if value := strings.TrimSpace(row.Merchant); value != "" && findOwnedAccount(value, accounts, 0) == nil {
			return value
		}
	}
	return ""
}

func resolveAccount(value string, accounts map[string]Account) (Account, bool) {
	if account, ok := accounts[normalizeName(value)]; ok {
		return account, true
	}
	if canonical := findOwnedAccount(value, accounts, 0); canonical != nil {
		return *canonical, true
	}
	return Account{}, false
}
func accountID(value *Account) int64 {
	if value == nil {
		return 0
	}
	return value.ID
}
func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
func unresolvedRow(row LegacyRow, reason string) Unresolved {
	return Unresolved{LegacyID: row.ID, Type: row.Type, Account: row.Account, Merchant: row.Merchant, Description: row.Description, TransferGroupID: row.TransferGroupID, Reason: reason}
}
func unresolvedGroup(groupID string, rows []LegacyRow, reason string) Unresolved {
	var representative LegacyRow
	if len(rows) > 0 {
		representative = rows[0]
	}
	return Unresolved{LegacyID: "transfer:" + groupID, Type: representative.Type, Account: representative.Account, Merchant: representative.Merchant, Description: representative.Description, TransferGroupID: groupID, Reason: reason}
}
func isSynthetic(row LegacyRow) bool {
	return strings.EqualFold(strings.TrimSpace(row.IsSynthetic), "true") || strings.TrimSpace(row.IsSynthetic) == "1"
}
func intPointer(value int64) *int64 { return &value }
func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	value = strings.TrimSpace(value)
	return &value
}
func parseConfidence(value string) *float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 0 || parsed > 1 {
		return nil
	}
	return &parsed
}
func normalizeParseStatus(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "AUTO", "RULE", "MANUAL", "NEEDS_REVIEW", "REPROCESS":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return "MANUAL"
	}
}
func countImported(summary *Summary, typ string) {
	if typ == "expense" {
		summary.ExpenseRowsImported++
	}
	if typ == "income" {
		summary.IncomeRowsImported++
	}
}
