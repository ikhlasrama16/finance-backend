package report

import (
	"sort"
	"strings"
	"time"
)

func Calculate(records []TransactionRecord, start, endExclusive time.Time) Statistics {
	statistics := Statistics{
		ExpenseByCategory: make([]CategoryTotal, 0),
		IncomeByCategory:  make([]CategoryTotal, 0),
		TopMerchants:      make([]MerchantTotal, 0),
		TopIncomeSources:  make([]MerchantTotal, 0),
	}
	expenseCategories := make(map[string]int64)
	incomeCategories := make(map[string]int64)
	merchants := make(map[string]MerchantTotal)
	incomeSources := make(map[string]MerchantTotal)

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
			category := strings.TrimSpace(record.CategoryName)
			if category == "" {
				category = "Belum Dikategorikan"
			}
			incomeCategories[category] += record.Amount
			merchant := strings.TrimSpace(record.Merchant)
			if merchant != "" {
				total := incomeSources[merchant]
				total.Merchant = merchant
				total.Amount += record.Amount
				total.TransactionCount++
				incomeSources[merchant] = total
			}
		case "expense":
			statistics.Summary.Expense += record.Amount
			statistics.Summary.TransactionCount++
			statistics.Summary.ExpenseTransactionCount++
			category := strings.TrimSpace(record.CategoryName)
			if category == "" {
				category = "Belum Dikategorikan"
			}
			expenseCategories[category] += record.Amount
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

	// 1. Expense by category
	for category, amount := range expenseCategories {
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

	// 2. Income by category
	for category, amount := range incomeCategories {
		percentage := 0.0
		if statistics.Summary.Income > 0 {
			percentage = float64(amount) * 100 / float64(statistics.Summary.Income)
		}
		statistics.IncomeByCategory = append(statistics.IncomeByCategory, CategoryTotal{Category: category, Amount: amount, Percentage: percentage})
	}
	sort.Slice(statistics.IncomeByCategory, func(i, j int) bool {
		if statistics.IncomeByCategory[i].Amount == statistics.IncomeByCategory[j].Amount {
			return statistics.IncomeByCategory[i].Category < statistics.IncomeByCategory[j].Category
		}
		return statistics.IncomeByCategory[i].Amount > statistics.IncomeByCategory[j].Amount
	})

	// 3. Top expense merchants
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

	// 4. Top income sources
	for _, source := range incomeSources {
		statistics.TopIncomeSources = append(statistics.TopIncomeSources, source)
	}
	sort.Slice(statistics.TopIncomeSources, func(i, j int) bool {
		if statistics.TopIncomeSources[i].Amount == statistics.TopIncomeSources[j].Amount {
			if statistics.TopIncomeSources[i].TransactionCount == statistics.TopIncomeSources[j].TransactionCount {
				return statistics.TopIncomeSources[i].Merchant < statistics.TopIncomeSources[j].Merchant
			}
			return statistics.TopIncomeSources[i].TransactionCount > statistics.TopIncomeSources[j].TransactionCount
		}
		return statistics.TopIncomeSources[i].Amount > statistics.TopIncomeSources[j].Amount
	})
	if len(statistics.TopIncomeSources) > 5 {
		statistics.TopIncomeSources = statistics.TopIncomeSources[:5]
	}

	return statistics
}

func BuildComparison(current, previous Statistics) Comparison {
	expenseChange := current.Summary.Expense - previous.Summary.Expense
	expensePct := 0.0
	if previous.Summary.Expense != 0 {
		expensePct = float64(expenseChange) * 100 / float64(previous.Summary.Expense)
	}

	incomeChange := current.Summary.Income - previous.Summary.Income
	incomePct := 0.0
	if previous.Summary.Income != 0 {
		incomePct = float64(incomeChange) * 100 / float64(previous.Summary.Income)
	}

	netChange := current.Summary.NetCashflow - previous.Summary.NetCashflow

	return Comparison{
		PreviousPeriodExpense:   previous.Summary.Expense,
		ExpenseChangeAmount:     expenseChange,
		ExpenseChangePercentage: expensePct,
		PreviousPeriodIncome:    previous.Summary.Income,
		IncomeChangeAmount:      incomeChange,
		IncomeChangePercentage:  incomePct,
		NetCashflowChangeAmount: netChange,
	}
}
