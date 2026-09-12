package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"finance-monitor/backend/internal/account"
	"finance-monitor/backend/internal/auth"
	"finance-monitor/backend/internal/category"
	"finance-monitor/backend/internal/middleware"
	"finance-monitor/backend/internal/notification"
	"finance-monitor/backend/internal/report"
	"finance-monitor/backend/internal/rule"
	"finance-monitor/backend/internal/transaction"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	httpServer   *http.Server
	db           *pgxpool.Pool
	reportWorker *report.Worker
}

type Options struct {
	IngestAPIKey              string
	AppEnv                    string
	CORSAllowedOrigins        []string
	OpenRouterAPIKey          string
	OpenRouterModel           string
	OpenRouterClassifierModel string
	AuthBootstrapEmail        string
	AuthBootstrapPassword     string
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
	aiFallbackParser := notification.NewOpenRouterAIFallbackParser(options.OpenRouterAPIKey, classifierModel)

	notificationService := notification.NewProcessingService(
		db,
		notificationRepository,
		accountRepository,
		categoryRepository,
		transactionRepository,
		ruleRepository,
	).WithClassifier(classifier).WithAIFallbackParser(aiFallbackParser)

	notificationHandler := notification.NewHandler(
		notificationService,
	)
	reportRepository := report.NewRepository(db)
	authRepository := auth.NewRepository(db)
	authService := auth.NewService(authRepository)
	authHandler := auth.NewHandler(authService, options.AppEnv == "production")
	reportClient := report.NewOpenRouterClient(options.OpenRouterAPIKey, options.OpenRouterModel)
	reportService := report.NewService(reportRepository, reportClient)
	reportHandler := report.NewHandler(reportService)
	reportV2Service := report.NewV2Service(reportRepository, reportClient)
	reportV2Handler := report.NewV2Handler(reportV2Service)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"Finance API","status":"running"}`)
	})

	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("GET /api/v1/ready", readinessHandler(db.Ping))
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/v1/auth/me", authHandler.Me)

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

	mux.HandleFunc("GET /api/v1/transactions/{id}", transactionHandler.GetByID)
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
	mux.HandleFunc("GET /api/v2/reports/statistics", reportV2Handler.Statistics)
	mux.HandleFunc("POST /api/v2/reports/ai", reportV2Handler.CreateAIJob)
	mux.HandleFunc("GET /api/v2/reports/ai/{id}", reportV2Handler.GetAIJob)
	mux.HandleFunc("GET /api/v1/notifications", notificationHandler.List)
	var notificationRoute http.Handler = http.HandlerFunc(notificationHandler.Create)
	notificationRoute = middleware.BearerAuth(options.IngestAPIKey)(notificationRoute)
	mux.Handle("POST /api/v1/notifications", notificationRoute)

	development := options.AppEnv != "production"
	var handler http.Handler = auth.RequireSession(authService)(mux)
	handler = middleware.CORS(options.CORSAllowedOrigins, development)(handler)

	return &Server{
		httpServer: &http.Server{
			Addr:              ":" + port,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		db: db, reportWorker: report.NewWorker(reportRepository, reportClient),
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
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	if s.reportWorker != nil {
		go s.reportWorker.Run(workerCtx)
	}
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
