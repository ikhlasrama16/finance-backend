package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const promptVersion = "v2"

const systemPrompt = `Anda adalah asisten analisis keuangan pribadi yang cerdas, objektif, dan suportif. Jawab dalam bahasa Indonesia.
Gunakan HANYA statistik dan data yang diberikan. Jangan mengarang transaksi atau angka baru, dan jangan mengubah total atau nilai yang telah disediakan.
Analisis secara seimbang kedua sisi: PEMASUKAN dan PENGELUARAN, serta arus kas bersih (net cashflow / surplus-defisit).
Jelaskan pola dan observasi secara ringkas, profesional, tanpa bahasa menghakimi.
Susun jawaban secara terstruktur dengan format Markdown yang rapi:
1. **Ringkasan Arus Kas**: Evaluasi rasio pemasukan vs pengeluaran dan surplus/defisit periode ini.
2. **Analisis Pemasukan**: Sumber pemasukan utama, kestabilan, dan perbandingannya jika ada data periode lalu.
3. **Pola Pengeluaran & Merchant**: Kategori dan penerima/merchant pengeluaran terbesar yang dominan.
4. **Hal yang Perlu Diperhatikan**: Anomali, defisit, atau ketergantungan belanja pada pos tertentu.
5. **Rekomendasi Praktis**: Langkah konkret dan realistis untuk mengoptimalkan kesehatan finansial pengguna.`

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
	PreviousPeriodIncome    int64           `json:"previous_period_income,omitempty"`
	IncomeChangeAmount      int64           `json:"income_change_amount,omitempty"`
	IncomeChangePercentage  float64         `json:"income_change_percentage,omitempty"`
	NetCashflowChangeAmount int64           `json:"net_cashflow_change_amount,omitempty"`
	ExpenseByCategory        []CategoryTotal `json:"expense_by_category"`
	IncomeByCategory         []CategoryTotal `json:"income_by_category,omitempty"`
	TopMerchants             []MerchantTotal `json:"top_merchants"`
	TopIncomeSources         []MerchantTotal `json:"top_income_sources,omitempty"`
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
		PreviousPeriodIncome:    response.Comparison.PreviousPeriodIncome,
		IncomeChangeAmount:      response.Comparison.IncomeChangeAmount,
		IncomeChangePercentage:  response.Comparison.IncomeChangePercentage,
		NetCashflowChangeAmount: response.Comparison.NetCashflowChangeAmount,
		ExpenseByCategory:        response.ExpenseByCategory,
		IncomeByCategory:         response.IncomeByCategory,
		TopMerchants:             response.TopMerchants,
		TopIncomeSources:         response.TopIncomeSources,
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
