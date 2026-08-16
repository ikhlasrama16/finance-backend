package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const promptVersion = "v1"

const systemPrompt = `Anda adalah asisten laporan keuangan pribadi. Jawab dalam bahasa Indonesia.
Gunakan hanya statistik yang diberikan. Jangan mengarang transaksi atau nominal, dan jangan menghitung ulang atau mengubah total yang diberikan.
Jelaskan pola dan observasi secara ringkas, tanpa bahasa menghakimi atau kepastian tentang motivasi pengguna.
Bedakan observasi dan saran praktis. Susun jawaban dengan bagian: Ringkasan, Pola utama, Hal yang perlu diperhatikan, dan Saran praktis.`

type promptData struct {
	Period                   string          `json:"period"`
	Income                   int64           `json:"income"`
	Expense                  int64           `json:"expense"`
	NetCashflow              int64           `json:"net_cashflow"`
	TransactionCount         int64           `json:"transaction_count"`
	AverageDailyExpense      int64           `json:"average_daily_expense"`
	ReconciliationAdjustment int64           `json:"reconciliation_adjustment"`
	PreviousPeriodExpense    int64           `json:"previous_period_expense"`
	ExpenseChangeAmount      int64           `json:"expense_change_amount"`
	ExpenseChangePercentage  float64         `json:"expense_change_percentage"`
	ExpenseByCategory        []CategoryTotal `json:"expense_by_category"`
	TopMerchants             []MerchantTotal `json:"top_merchants"`
}

func BuildPrompt(response Response) (string, error) {
	data := promptData{
		Period:                   formatPeriod(response.StartDate, response.EndDate),
		Income:                   response.Summary.Income,
		Expense:                  response.Summary.Expense,
		NetCashflow:              response.Summary.NetCashflow,
		TransactionCount:         response.Summary.TransactionCount,
		AverageDailyExpense:      response.Summary.AverageDailyExpense,
		ReconciliationAdjustment: response.Summary.ReconciliationAdjustment,
		PreviousPeriodExpense:    response.Comparison.PreviousPeriodExpense,
		ExpenseChangeAmount:      response.Comparison.ExpenseChangeAmount,
		ExpenseChangePercentage:  response.Comparison.ExpenseChangePercentage,
		ExpenseByCategory:        response.ExpenseByCategory,
		TopMerchants:             response.TopMerchants,
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("encode AI report prompt: %w", err)
	}
	return "Berikut statistik keuangan teragregasi yang harus Anda jelaskan:\n" + string(encoded), nil
}

func SummaryHash(response Response) (string, error) {
	value := struct {
		Version  string   `json:"version"`
		Response Response `json:"response"`
	}{Version: promptVersion, Response: response}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode report summary hash: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func formatPeriod(startDate, endDate string) string {
	start, startErr := time.Parse("2006-01-02", startDate)
	end, endErr := time.Parse("2006-01-02", endDate)
	if startErr != nil || endErr != nil {
		return startDate + " sampai " + endDate
	}
	months := []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	if start.Year() == end.Year() && start.Month() == end.Month() {
		return fmt.Sprintf("%d-%d %s %d", start.Day(), end.Day(), months[start.Month()-1], start.Year())
	}
	return fmt.Sprintf("%d %s %d sampai %d %s %d", start.Day(), months[start.Month()-1], start.Year(), end.Day(), months[end.Month()-1], end.Year())
}
