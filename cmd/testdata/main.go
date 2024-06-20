package main

import (
	"context"

	"github.com/gesundheitscloud/go-svc-template/internal/testutils"
	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/db"
	"github.com/gesundheitscloud/go-svc/pkg/standard"
	"github.com/spf13/viper"
)

func main() {
	// Initialize the environment and logger
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
	standard.Main(addTestData, "go-svc-template-testdata", standard.WithPostgres(dbOpts))
}

func addTestData(_ context.Context, _ string) <-chan struct{} {
	// Insert test data
	testutils.AddTestDataExamplesToDB()

	dieEarly := make(chan struct{})
	close(dieEarly)
	return dieEarly
}
