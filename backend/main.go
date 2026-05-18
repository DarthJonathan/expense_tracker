package main

import (
	_ "embed"
	"net/http"
	"os"
	"time"

	_ "expense-tracker/backend/docs"

	"expense-tracker/backend/config"
	"expense-tracker/backend/constants"
	"expense-tracker/backend/controllers"
	"expense-tracker/backend/database"
	"expense-tracker/backend/middleware"
	"expense-tracker/backend/service"

	"github.com/apex/log"
	textHandler "github.com/apex/log/handlers/text"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/ilyakaznacheev/cleanenv"
	httpSwagger "github.com/swaggo/http-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

//go:embed static/signup.html
var signupHTML string

func RunServer() error {
	if err := cleanenv.ReadConfig("config.yml", &config.Cfg); err != nil {
		log.WithError(err).Warn("config.yml not found or invalid, reading env only")
		_ = cleanenv.ReadEnv(&config.Cfg)
	}

	log.SetHandler(textHandler.New(os.Stdout))
	log.SetLevelFromString(config.Cfg.Server.LoggingLevel)

	db := initDatabase(config.Cfg.Database.URL)
	if err := database.Migrate(db); err != nil {
		log.WithError(err).Fatal("failed to run database migrations")
	}
	router := initRouter(db)

	return initHTTPServer(config.Cfg.Server.Port, config.Cfg.Cors.AllowOrigin, router)
}

func initHTTPServer(servingPort string, allowOrigin string, router *mux.Router) error {
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization", "X-API-Key", "X-Request-Id"})
	originsOk := handlers.AllowedOrigins([]string{allowOrigin})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"})
	allowCreds := handlers.AllowCredentials()

	log.WithFields(log.Fields{"port": servingPort, "cors": allowOrigin}).Info("HTTP server started")
	return http.ListenAndServe(":"+servingPort, handlers.CORS(originsOk, headersOk, methodsOk, allowCreds)(router))
}

func initRouter(db *gorm.DB) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	router.Use(middleware.RequestIDMiddleware)
	router.Use(accessLogMiddleware)

	syncService := service.NewSyncService(db)
	syncController := controllers.NewSyncController(syncService)
	expenseService := service.NewExpenseService(db)
	expenseController := controllers.NewExpenseController(expenseService)
	authService := service.NewAuthService(db, config.Cfg.Auth.JWTSecret, config.Cfg.Auth.TokenExpiry)
	authController := controllers.NewAuthController(authService)

	router.HandleFunc("/healthz", syncController.HealthV1).Methods(http.MethodGet)
	router.HandleFunc("/signup", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(signupHTML))
	}).Methods(http.MethodGet)
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"service":"expense-tracker-backend","docs":"/swagger/index.html","health":"/healthz"}`))
	}).Methods(http.MethodGet)
	router.HandleFunc("/api/v1/auth/register", authController.RegisterV1).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/v1/auth/login", authController.LoginV1).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/v1/auth/api-keys/authenticate", authController.AuthenticateAPIKeyV1).Methods(http.MethodPost, http.MethodOptions)

	protected := router.PathPrefix("/api/v1").Subrouter()
	protected.Use(middleware.AuthMiddleware(authService))
	protected.HandleFunc("/auth/api-keys", authController.CreateAPIKeyV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/sync", syncController.SyncV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/groups", expenseController.CreateGroupV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/groups/{groupId}/accounts", expenseController.ListAccountsV1).Methods(http.MethodGet)
	protected.HandleFunc("/groups/{groupId}/accounts", expenseController.CreateAccountV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/groups/{groupId}/categories", expenseController.ListCategoriesV1).Methods(http.MethodGet)
	protected.HandleFunc("/groups/{groupId}/categories", expenseController.CreateCategoryV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/groups/{groupId}/expenses", expenseController.ListExpensesV1).Methods(http.MethodGet)
	protected.HandleFunc("/groups/{groupId}/expenses", expenseController.CreateExpenseV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/entries", expenseController.CreateEntryV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/groups/{groupId}/transactions", expenseController.ListExpensesV1).Methods(http.MethodGet)
	protected.HandleFunc("/groups/{groupId}/transactions", expenseController.CreateExpenseV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/groups/{groupId}/adjustments", expenseController.ListAdjustmentsV1).Methods(http.MethodGet)
	protected.HandleFunc("/groups/{groupId}/adjustments", expenseController.CreateAdjustmentV1).Methods(http.MethodPost, http.MethodOptions)
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	return router
}

func initDatabase(databaseURL string) *gorm.DB {
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  databaseURL,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 gormLogger.Default.LogMode(gormLogger.Silent),
	})
	if err != nil {
		log.WithError(err).Fatal("failed to initialize database")
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.WithError(err).Fatal("failed to retrieve sql db")
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Info("database connected")
	return db
}

func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)

		requestID, _ := r.Context().Value(constants.RequestIDCtx).(string)
		log.WithFields(log.Fields{
			"request_id":  requestID,
			"method":      r.Method,
			"path":        r.URL.Path,
			"duration_ms": time.Since(startedAt).Milliseconds(),
		}).Info("request completed")
	})
}

func main() {
	if err := RunServer(); err != nil {
		log.WithError(err).Error("error starting app")
		os.Exit(1)
	}
}
