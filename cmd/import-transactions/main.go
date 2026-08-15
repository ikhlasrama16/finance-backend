package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"finance-monitor/backend/internal/database"
	"finance-monitor/backend/internal/importer"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	filePath := flag.String("file", "", "legacy transaction CSV path")
	dryRun := flag.Bool("dry-run", false, "parse and resolve without inserting")
	createMissing := flag.Bool("create-missing-categories", false, "create missing income/expense categories during import")
	flag.Parse()
	if *filePath == "" {
		log.Fatal("-file is required")
	}

	file, err := os.Open(*filePath)
	if err != nil {
		log.Fatalf("open CSV: %v", err)
	}
	rows, err := importer.ReadCSV(file)
	file.Close()
	if err != nil {
		log.Fatal(err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx := context.Background()
	db, err := database.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	if *dryRun {
		accounts, categories, err := importer.LoadReferences(ctx, db)
		if err != nil {
			log.Fatal(err)
		}
		resolved := importer.Normalize(rows, accounts, categories)
		printSummary(resolved.Summary)
		return
	}

	if err := runImport(ctx, db, rows, *createMissing); err != nil {
		log.Fatal(err)
	}
}

func runImport(ctx context.Context, db *pgxpool.Pool, rows []importer.LegacyRow, createMissing bool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin import transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if createMissing {
		if err := importer.EnsureMissingCategories(ctx, tx, rows); err != nil {
			return err
		}
	}
	accounts, categories, err := importer.LoadReferences(ctx, tx)
	if err != nil {
		return err
	}
	resolved := importer.Normalize(rows, accounts, categories)
	for _, value := range resolved.Transactions {
		inserted, err := importer.Insert(ctx, tx, value)
		if err != nil {
			return err
		}
		if !inserted {
			resolved.Summary.DuplicatesSkipped++
			if value.Type == "expense" {
				resolved.Summary.ExpenseRowsImported--
			} else if value.Type == "income" {
				resolved.Summary.IncomeRowsImported--
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit import transaction: %w", err)
	}
	printSummary(resolved.Summary)
	return nil
}

func printSummary(summary importer.Summary) {
	fmt.Printf("rows read: %d\n", summary.RowsRead)
	fmt.Printf("expense rows imported: %d\n", summary.ExpenseRowsImported)
	fmt.Printf("income rows imported: %d\n", summary.IncomeRowsImported)
	fmt.Printf("transfers collapsed: %d\n", summary.TransfersCollapsed)
	fmt.Printf("synthetic rows ignored: %d\n", summary.SyntheticRowsIgnored)
	fmt.Printf("unknown rows skipped: %d\n", summary.UnknownRowsSkipped)
	fmt.Printf("marketplace support rows skipped: %d\n", summary.MarketplaceSupportSkipped)
	fmt.Printf("external transfer-ins converted to income: %d\n", summary.ExternalTransferInsConverted)
	fmt.Printf("external transfer-outs converted to expense: %d\n", summary.ExternalTransferOutsConverted)
	fmt.Printf("unresolved rows: %d\n", len(summary.UnresolvedRows))
	fmt.Printf("duplicates skipped: %d\n", summary.DuplicatesSkipped)
	fmt.Printf("errors: %d\n", summary.Errors)
	for _, unresolved := range summary.UnresolvedRows {
		fmt.Printf("unresolved legacy_id=%s type=%s account=%s merchant=%s description=%s transfer_group_id=%s reason=%s\n", unresolved.LegacyID, unresolved.Type, unresolved.Account, unresolved.Merchant, unresolved.Description, unresolved.TransferGroupID, unresolved.Reason)
	}
}
