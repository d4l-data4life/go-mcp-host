package models

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc/pkg/db2"
)

// InitializeTestDB connects to an inmemory sqlite for testing
func InitializeTestDB(t *testing.T) {
	config.SetupEnv()
	config.SetupLogger()
	dbOpts := db2.NewConnection(
		db2.WithDebug(viper.GetBool("DEBUG")),
		db2.WithHost(viper.GetString("DB_HOST")),
		db2.WithPort(viper.GetString("DB_PORT")),
		db2.WithDatabaseName(viper.GetString("DB_NAME")),
		db2.WithUser(viper.GetString("DB_USER")),
		db2.WithPassword(viper.GetString("DB_PASS")),
		db2.WithSSLMode(viper.GetString("DB_SSL_MODE")),
		db2.WithSSLRootCertPath(viper.GetString("DB_SSL_ROOT_CERT_PATH")),
		db2.WithMigrationFunc(MigrationFunc),
		db2.WithDriverFunc(db2.TXDBPostgresDriver),
	)
	db2.InitializeTestPostgres(dbOpts)
	assert.NotNil(t, db2.Get(), "DB handle is nil")
	err := db2.Ping()
	assert.NoError(t, err)
}
