package models

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc/pkg/db"
)

// InitializeTestDB connects to an inmemory sqlite for testing
func InitializeTestDB(t *testing.T) {
	config.SetupEnv()
	config.SetupLogger()
	// override schema for testing
	viper.Set("DB_SCHEMA", "testing")
	dbOpts := db.NewConnection(
		db.WithDebug(viper.GetBool("DEBUG")),
		db.WithHost(viper.GetString("DB_HOST")),
		db.WithPort(viper.GetString("DB_PORT")),
		db.WithDatabaseName(viper.GetString("DB_NAME")),
		db.WithDatabaseSchema(viper.GetString("DB_SCHEMA")),
		db.WithUser(viper.GetString("DB_USER")),
		db.WithPassword(viper.GetString("DB_PASS")),
		db.WithSSLMode(viper.GetString("DB_SSL_MODE")),
		db.WithSSLRootCertPath(viper.GetString("DB_SSL_ROOT_CERT_PATH")),
		db.WithMigrationFunc(MigrationFunc),
		db.WithDriverFunc(db.TXDBPostgresDriver),
	)
	db.InitializeTestPostgres(dbOpts)
	assert.NotNil(t, db.Get(), "DB handle is nil")
	err := db.Ping()
	assert.NoError(t, err)
}
