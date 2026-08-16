package report

import (
	"sort"
	"strings"
	"time"
)

func Calculate(records []TransactionRecord, start, endExclusive time.Time) Statistics {
	statistics := Statistics{ExpenseByCategory: make([]CategoryTotal, 0), TopMerchants: make([]MerchantTotal, 0)}
	categories := make(map[string]int64)
	merchants := make(map[string]MerchantTotal)

	for _, record := range records {
		if record.Source == "reconcile" {
			switch record.Type {
			case "income":
				statistics.Summary.ReconciliationAdjustment += record.Amount
			case "expense":
				statistics.Summary.ReconciliationAdjustment -= record.Amount
			}
			continue
		}

		switch record.Type {
		case "income":
			statistics.Summary.Income += record.Amount
			statistics.Summary.TransactionCount++
		case "expense":
			statistics.Summary.Expense += record.Amount
			statistics.Summary.TransactionCount++
			statistics.Summary.ExpenseTransactionCount++
			category := strings.TrimSpace(record.CategoryName)
			if category == "" {
				category = "Belum Dikategorikan"
			}
			categories[category] += record.Amount
			merchant := strings.TrimSpace(record.Merchant)
			if merchant != "" {
				total := merchants[merchant]
				total.Merchant = merchant
				total.Amount += record.Amount
				total.TransactionCount++
				merchants[merchant] = total
			}
		case "transfer":
			statistics.Summary.TransactionCount++
			statistics.Summary.TransferCount++
		}
	}

	statistics.Summary.NetCashflow = statistics.Summary.Income - statistics.Summary.Expense
	days := int(endExclusive.Sub(start).Hours() / 24)
	if days > 0 {
		statistics.Summary.AverageDailyExpense = statistics.Summary.Expense / int64(days)
	}
	for category, amount := range categories {
		percentage := 0.0
		if statistics.Summary.Expense > 0 {
			percentage = float64(amount) * 100 / float64(statistics.Summary.Expense)
		}
		statistics.ExpenseByCategory = append(statistics.ExpenseByCategory, CategoryTotal{Category: category, Amount: amount, Percentage: percentage})
	}
	sort.Slice(statistics.ExpenseByCategory, func(i, j int) bool {
		if statistics.ExpenseByCategory[i].Amount == statistics.ExpenseByCategory[j].Amount {
			return statistics.ExpenseByCategory[i].Category < statistics.ExpenseByCategory[j].Category
		}
		return statistics.ExpenseByCategory[i].Amount > statistics.ExpenseByCategory[j].Amount
	})
	for _, merchant := range merchants {
		statistics.TopMerchants = append(statistics.TopMerchants, merchant)
	}
	sort.Slice(statistics.TopMerchants, func(i, j int) bool {
		if statistics.TopMerchants[i].Amount == statistics.TopMerchants[j].Amount {
			if statistics.TopMerchants[i].TransactionCount == statistics.TopMerchants[j].TransactionCount {
				return statistics.TopMerchants[i].Merchant < statistics.TopMerchants[j].Merchant
			}
			return statistics.TopMerchants[i].TransactionCount > statistics.TopMerchants[j].TransactionCount
		}
		return statistics.TopMerchants[i].Amount > statistics.TopMerchants[j].Amount
	})
	if len(statistics.TopMerchants) > 5 {
		statistics.TopMerchants = statistics.TopMerchants[:5]
	}
	return statistics
}

func BuildComparison(current, previous Statistics) Comparison {
	change := current.Summary.Expense - previous.Summary.Expense
	percentage := 0.0
	if previous.Summary.Expense != 0 {
		percentage = float64(change) * 100 / float64(previous.Summary.Expense)
	}
	return Comparison{PreviousPeriodExpense: previous.Summary.Expense, ExpenseChangeAmount: change, ExpenseChangePercentage: percentage}
}
