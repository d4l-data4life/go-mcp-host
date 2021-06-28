package main

import (
	"context"
	"os"
	"strings"

	"github.com/go-chi/cors"
	"github.com/spf13/viper"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc-template/pkg/metrics"
	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc-template/pkg/server"
	"github.com/gesundheitscloud/go-svc/pkg/db"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
	"github.com/gesundheitscloud/go-svc/pkg/standard"
)

func main() {
	config.SetupEnv()

	logging.LoggerConfig(
		logging.ServiceName("go-svc-template"),
		logging.ServiceVersion(config.Version),
		logging.Hostname(os.Getenv("HOSTNAME")),
		logging.PodName(os.Getenv("POD_NAME")),
		logging.Environment(os.Getenv("ENVIRONMENT")),
		logging.Debug(viper.GetBool("DEBUG")),
		logging.HumanReadable(viper.GetBool("HUMAN_READABLE_LOGS")),
	)
	dbOpts := db.NewConnection(
		db.WithDebug(viper.GetBool("DEBUG")),
		db.WithMaxConnectionLifetime(viper.GetDuration("DB_MAX_CONNECTION_LIFETIME")),
		db.WithMaxIdleConnections(viper.GetInt("DB_MAX_IDLE_CONNECTIONS")),
		db.WithMaxOpenConnections(viper.GetInt("DB_MAX_OPEN_CONNECTIONS")),
		db.WithHost(viper.GetString("DB_HOST")),
		db.WithPort(viper.GetString("DB_PORT")),
		db.WithDatabaseName(viper.GetString("DB_NAME")),
		db.WithUser(viper.GetString("DB_USER")),
		db.WithPassword(viper.GetString("DB_PASS")),
		db.WithSSLMode(viper.GetString("DB_SSL_MODE")),
		db.WithMigrationFunc(models.MigrationFunc),
	)
	standard.Main(mainAPI, "go-svc-template", standard.WithPostgres(dbOpts))
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

	if err := config.LoadJWTPublicKey(); err != nil {
		logging.LogErrorfCtx(runCtx, err, "error loading JWT public key")
		return dieEarly
	}
	server.SetupRoutes(srv.Mux())
	metrics.AddBuildInfoMetric()
	return standard.ListenAndServe(runCtx, srv.Mux(), port)
}
