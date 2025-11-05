package main

import (
	"context"
	"encoding/base64"
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

	var jwtSecret []byte

	// Initialize TokenValidator
	var tokenValidator auth.TokenValidator
	remoteKeysURL := viper.GetString("REMOTE_KEYS_URL")
	if remoteKeysURL != "" {
		// Use remote token validator for external authentication
		logging.LogInfof("Initializing remote token validator with URL: %s", remoteKeysURL)
		var err error
		tokenValidator, err = auth.NewRemoteKeyStore(context.Background(), remoteKeysURL)
		if err != nil {
			logging.LogErrorf(err, "failed to create remote key store")
			return dieEarly
		}
		logging.LogInfof("Remote token validator initialized successfully")
	} else {
		// Use local JWT validator for internal authentication
		logging.LogInfof("REMOTE_KEYS_URL not configured - using local JWT authentication")

		// Get JWT secret from environment (base64-encoded)
		jwtSecretB64 := viper.GetString("JWT_SECRET")
		if jwtSecretB64 == "" {
			logging.LogErrorf(nil, "JWT_SECRET environment variable is not set")
			return dieEarly
		}

		var err error
		jwtSecret, err = base64.StdEncoding.DecodeString(jwtSecretB64)
		if err != nil {
			logging.LogErrorf(err, "failed to decode JWT secret from base64")
			return dieEarly
		}

		tokenValidator, err = auth.NewLocalJWTValidator(jwtSecret)
		if err != nil {
			logging.LogErrorf(err, "failed to create local JWT validator")
			return dieEarly
		}
		logging.LogInfof("Local JWT validator initialized successfully")
	}

	server.SetupRoutes(runCtx, srv.Mux(), tokenValidator, jwtSecret)
	metrics.AddBuildInfoMetric()
	return standard.ListenAndServe(runCtx, srv.Mux(), port)
}
