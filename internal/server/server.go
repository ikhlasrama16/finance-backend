package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"finance-monitor/backend/internal/account"
	"finance-monitor/backend/internal/category"
	"finance-monitor/backend/internal/middleware"
	"finance-monitor/backend/internal/notification"
	"finance-monitor/backend/internal/report"
	"finance-monitor/backend/internal/rule"
	"finance-monitor/backend/internal/transaction"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	httpServer *http.Server
	db         *pgxpool.Pool
}

type Options struct {
	IngestAPIKey              string
	AppEnv                    string
	CORSAllowedOrigins        []string
	OpenRouterAPIKey          string
	OpenRouterModel           string
	OpenRouterClassifierModel string
}

func New(
	port string,
	db *pgxpool.Pool,
) *Server {
	return NewWithOptions(port, db, Options{})
}

func NewWithOptions(port string, db *pgxpool.Pool, options Options) *Server {
	mux := http.NewServeMux()

	accountRepository := account.NewRepository(db)
	accountService := account.NewService(accountRepository)
	accountHandler := account.NewHandler(accountService)
	categoryRepository := category.NewRepository(db)

	ruleRepository := rule.NewRepository(db)
	transactionRepository := transaction.NewRepository(db)
	transactionService := transaction.NewService(
		transactionRepository,
		categoryRepository,
	).WithRuleRepository(ruleRepository)
	transactionHandler := transaction.NewHandler(transactionService)

	categoryService := category.NewService(categoryRepository)
	categoryHandler := category.NewHandler(categoryService)

	notificationRepository := notification.NewRepository(db)

	classifierModel := options.OpenRouterClassifierModel
	if classifierModel == "" {
		classifierModel = options.OpenRouterModel
	}
	classifier := category.NewOpenRouterClassifier(options.OpenRouterAPIKey, classifierModel)

	notificationService := notification.NewProcessingService(
		db,
		notificationRepository,
		accountRepository,
		categoryRepository,
		transactionRepository,
		ruleRepository,
	).WithClassifier(classifier)

	notificationHandler := notification.NewHandler(
		notificationService,
	)
	reportRepository := report.NewRepository(db)
	reportClient := report.NewOpenRouterClient(options.OpenRouterAPIKey, options.OpenRouterModel)
	reportService := report.NewService(reportRepository, reportClient)
	reportHandler := report.NewHandler(reportService)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"Finance API","status":"running"}`)
	})

	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("GET /api/v1/ready", readinessHandler(db.Ping))

	mux.HandleFunc(
		"GET /api/v1/accounts",
		accountHandler.List,
	)

	mux.HandleFunc(
		"POST /api/v1/accounts",
		accountHandler.Create,
	)

	mux.HandleFunc(
		"POST /api/v1/accounts/{id}/reconcile",
		accountHandler.Reconcile,
	)

	mux.HandleFunc(
		"GET /api/v1/transactions",
		transactionHandler.List,
	)

	mux.HandleFunc(
		"POST /api/v1/transactions",
		transactionHandler.Create,
	)
	mux.HandleFunc("PATCH /api/v1/transactions/{id}", transactionHandler.Update)
	mux.HandleFunc("DELETE /api/v1/transactions/{id}", transactionHandler.Delete)

	mux.HandleFunc(
		"GET /api/v1/categories",
		categoryHandler.List,
	)

	mux.HandleFunc(
		"POST /api/v1/categories",
		categoryHandler.Create,
	)
	mux.HandleFunc("POST /api/v1/reports/ai", reportHandler.Create)
	var notificationRoute http.Handler = http.HandlerFunc(notificationHandler.Create)
	notificationRoute = middleware.BearerAuth(options.IngestAPIKey)(notificationRoute)
	mux.Handle("POST /api/v1/notifications", notificationRoute)

	development := options.AppEnv != "production"
	handler := middleware.CORS(options.CORSAllowedOrigins, development)(mux)

	return &Server{
		httpServer: &http.Server{
			Addr:              ":" + port,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		db: db,
	}
}

func readinessHandler(ping func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"status":"not_ready"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ready"}`)
	}
}

func (s *Server) Run() error {
	return s.RunContext(context.Background())
}

func (s *Server) RunContext(ctx context.Context) error {
	errors := make(chan error, 1)
	go func() { errors <- s.httpServer.ListenAndServe() }()
	select {
	case err := <-errors:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}
