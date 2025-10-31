package main

import (
	"context"
	"strings"

	"github.com/go-chi/cors"
	"github.com/spf13/viper"

	"github.com/d4l-data4life/go-mcp-host/pkg/auth"
	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/metrics"
	"github.com/d4l-data4life/go-mcp-host/pkg/models"
	"github.com/d4l-data4life/go-mcp-host/pkg/server"

	"github.com/d4l-data4life/go-svc/pkg/db"
	"github.com/d4l-data4life/go-svc/pkg/logging"
	"github.com/d4l-data4life/go-svc/pkg/standard"
)

func main() {
	config.SetupEnv()
	config.SetupLogger()
	dbOpts := db.NewConnection(
		db.WithDebug(viper.GetBool("DEBUG")),
		db.WithHost(viper.GetString("DB_HOST")),
		db.WithPort(viper.GetString("DB_PORT")),
		db.WithDatabaseSchema(viper.GetString("DB_SCHEMA")),
		db.WithDatabaseName(viper.GetString("DB_NAME")),
		db.WithUser(viper.GetString("DB_USER")),
		db.WithPassword(viper.GetString("DB_PASS")),
		db.WithSSLMode(viper.GetString("DB_SSL_MODE")),
		db.WithSSLRootCertPath(viper.GetString("DB_SSL_ROOT_CERT_PATH")),
		db.WithMigrationFunc(models.MigrationFunc),
		db.WithMigrationVersion(config.MigrationVersion),
	)
	standard.Main(mainAPI, "go-mcp-host", standard.WithPostgres(dbOpts))
}

// mainAPI contains the main service logic - it must finish on runCtx cancelation!
func mainAPI(runCtx context.Context, svcName string) <-chan struct{} {
	port := viper.GetString("PORT")
	corsOptions := config.CorsConfig(strings.Split(viper.GetString("CORS_HOSTS"), " "))
	srv := server.NewServer(svcName,
		cors.New(corsOptions),
		viper.GetInt("HTTP_MAX_PARALLEL_REQUESTS"),
		viper.GetDuration("HTTP_REQUEST_TIMEOUT"),
	)

	dieEarly := make(chan struct{})
	defer close(dieEarly)

	_, err := config.LoadJwtKey(viper.GetString("JWT_KEY_PATH"))
	if err != nil {
		logging.LogErrorf(err, "failed to load JWT key")
		return dieEarly
	}

	// Initialize TokenValidator if REMOTE_KEYS_URL is configured
	var tokenValidator auth.TokenValidator
	remoteKeysURL := viper.GetString("REMOTE_KEYS_URL")
	if remoteKeysURL != "" {
		logging.LogInfof("Initializing remote token validator with URL: %s", remoteKeysURL)
		tokenValidator, err = auth.NewRemoteKeyStore(context.Background(), remoteKeysURL)
		if err != nil {
			logging.LogErrorf(err, "failed to create remote key store - continuing without remote validation")
			tokenValidator = nil
		} else {
			logging.LogInfof("Remote token validator initialized successfully")
		}
	} else {
		logging.LogInfof("REMOTE_KEYS_URL not configured - using local JWT authentication")
	}

	server.SetupRoutes(runCtx, srv.Mux(), tokenValidator)
	metrics.AddBuildInfoMetric()
	return standard.ListenAndServe(runCtx, srv.Mux(), port)
}
