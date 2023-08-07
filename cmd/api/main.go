package main

import (
	"context"
	"strings"

	"github.com/go-chi/cors"
	"github.com/spf13/viper"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc-template/pkg/metrics"
	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc-template/pkg/server"
	"github.com/gesundheitscloud/go-svc/pkg/db2"
	"github.com/gesundheitscloud/go-svc/pkg/standard"
)

func main() {
	config.SetupEnv()
	config.SetupLogger()
	dbOpts := db2.NewConnection(
		db2.WithDebug(viper.GetBool("DEBUG")),
		db2.WithHost(viper.GetString("DB_HOST")),
		db2.WithPort(viper.GetString("DB_PORT")),
		db2.WithDatabaseSchema(viper.GetString("DB_SCHEMA")),
		db2.WithDatabaseName(viper.GetString("DB_NAME")),
		db2.WithUser(viper.GetString("DB_USER")),
		db2.WithPassword(viper.GetString("DB_PASS")),
		db2.WithSSLMode(viper.GetString("DB_SSL_MODE")),
		db2.WithSSLRootCertPath(viper.GetString("DB_SSL_ROOT_CERT_PATH")),
		db2.WithMigrationFunc(models.MigrationFunc),
		db2.WithMigrationVersion(config.MigrationVersion),
	)
	standard.Main(mainAPI, "go-svc-template", standard.WithPostgresDB2(dbOpts))
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

	server.SetupRoutes(srv.Mux())
	metrics.AddBuildInfoMetric()
	return standard.ListenAndServe(runCtx, srv.Mux(), port)
}
