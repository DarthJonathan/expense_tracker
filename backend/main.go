package main

import (
	_ "embed"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "expense-tracker/backend/docs"

	"expense-tracker/backend/config"
	"expense-tracker/backend/constants"
	"expense-tracker/backend/controllers"
	"expense-tracker/backend/dao"
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

	if err := dao.SetSchema(config.Cfg.Database.Schema); err != nil {
		log.WithError(err).Fatal("invalid database schema")
	}

	dbURL := resolveDatabaseURL()
	db := initDatabase(dbURL)
	if err := database.Migrate(db); err != nil {
		log.WithError(err).Fatal("failed to run database migrations")
	}
	router := initRouter(db)

	return initHTTPServer(config.Cfg.Server.Port, config.Cfg.Cors.AllowOrigin, router)
}

func initHTTPServer(servingPort string, allowOrigin string, router *mux.Router) error {
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization", "X-API-Key", "X-Request-Id"})
	originsOk := handlers.AllowedOrigins([]string{allowOrigin})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "PATCH", "OPTIONS"})
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
	protected.HandleFunc("/entries/automation", expenseController.CreateAutomationEntryV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/entries/apple", expenseController.CreateAutomationEntryV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/groups/{groupId}/transactions", expenseController.ListExpensesV1).Methods(http.MethodGet)
	protected.HandleFunc("/groups/{groupId}/transactions", expenseController.CreateExpenseV1).Methods(http.MethodPost, http.MethodOptions)
	protected.HandleFunc("/groups/{groupId}/transactions/{transactionId}", expenseController.UpdateExpenseV1).Methods(http.MethodPut, http.MethodPatch, http.MethodOptions)
	protected.HandleFunc("/groups/{groupId}/adjustments", expenseController.ListAdjustmentsV1).Methods(http.MethodGet)
	protected.HandleFunc("/groups/{groupId}/adjustments", expenseController.CreateAdjustmentV1).Methods(http.MethodPost, http.MethodOptions)
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	return router
}

func initDatabase(databaseURL string) *gorm.DB {
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required, or set DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME")
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
	registerMutationLogCallbacks(db)

	log.Info("database connected")
	return db
}

func registerMutationLogCallbacks(db *gorm.DB) {
	logMutation := func(operation string) func(*gorm.DB) {
		return func(tx *gorm.DB) {
			if tx == nil || tx.Statement == nil {
				return
			}
			if tx.Error != nil {
				log.WithError(tx.Error).WithField("operation", operation).Warn("database mutation failed")
				return
			}

			table := tx.Statement.Table
			if table == "" && tx.Statement.Schema != nil {
				table = tx.Statement.Schema.Table
			}
			if table == "" {
				table = "unknown"
			}

			log.WithFields(log.Fields{
				"operation":     operation,
				"table":         table,
				"rows_affected": tx.RowsAffected,
			}).Info("database mutation")
		}
	}

	_ = db.Callback().Create().After("gorm:create").Register("spendit:log_create", logMutation("insert"))
	_ = db.Callback().Update().After("gorm:update").Register("spendit:log_update", logMutation("update"))
	_ = db.Callback().Delete().After("gorm:delete").Register("spendit:log_delete", logMutation("delete"))
}

func resolveDatabaseURL() string {
	if directURL := strings.TrimSpace(config.Cfg.Database.URL); directURL != "" {
		return directURL
	}

	host := strings.TrimSpace(firstNonEmpty(
		os.Getenv("DB_HOST"),
		os.Getenv("DATABASE_HOST"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("PGHOST"),
	))
	port := strings.TrimSpace(firstNonEmpty(
		os.Getenv("DB_PORT"),
		os.Getenv("DATABASE_PORT"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("PGPORT"),
	))
	user := strings.TrimSpace(firstNonEmpty(
		os.Getenv("DB_USER"),
		os.Getenv("DB_USERNAME"),
		os.Getenv("DATABASE_USER"),
		os.Getenv("DATABASE_USERNAME"),
		os.Getenv("POSTGRES_USER"),
		os.Getenv("PGUSER"),
	))
	password := firstNonEmpty(
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DATABASE_PASSWORD"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("PGPASSWORD"),
	)
	name := strings.TrimSpace(firstNonEmpty(
		os.Getenv("DB_NAME"),
		os.Getenv("DB_DATABASE"),
		os.Getenv("DATABASE_NAME"),
		os.Getenv("DATABASE_DB"),
		os.Getenv("POSTGRES_DB"),
		os.Getenv("PGDATABASE"),
	))
	sslMode := strings.TrimSpace(firstNonEmpty(os.Getenv("DB_SSLMODE"), "disable"))

	if host == "" || user == "" || name == "" {
		return ""
	}
	if port == "" {
		port = "5432"
	}

	dsn := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + name,
	}
	if password != "" {
		dsn.User = url.UserPassword(user, password)
	} else {
		dsn.User = url.User(user)
	}

	query := dsn.Query()
	query.Set("sslmode", sslMode)
	if schema := strings.TrimSpace(config.Cfg.Database.Schema); schema != "" {
		query.Set("search_path", schema)
	}

	if extraOptions := strings.TrimSpace(os.Getenv("DB_OPTIONS")); extraOptions != "" {
		// Support comma-separated key=value pairs in DB_OPTIONS.
		parts := strings.Split(extraOptions, ",")
		for _, part := range parts {
			option := strings.TrimSpace(part)
			if option == "" {
				continue
			}
			kv := strings.SplitN(option, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			if key != "" && value != "" {
				query.Set(key, value)
			}
		}
	}

	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
