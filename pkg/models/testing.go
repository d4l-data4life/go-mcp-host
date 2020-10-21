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
	if viper.IsSet("DB_HOST") {
		dbOpts := db.NewConnection(
			db.WithDebug(viper.GetBool("DEBUG")),
			db.WithHost(viper.GetString("DB_HOST")),
			db.WithPort(viper.GetString("DB_PORT")),
			db.WithDatabaseName(viper.GetString("DB_NAME")),
			db.WithUser(viper.GetString("DB_USER")),
			db.WithPassword(viper.GetString("DB_PASS")),
			db.WithSSLMode(viper.GetString("DB_SSL_MODE")),
			db.WithMigrationFunc(MigrationFunc),
			db.WithDriverFunc(db.TXDBPostgresDriver),
		)
		db.InitializeTestPostgres(dbOpts)
		assert.NotNil(t, db.Get(), "DB handle is nil")
		err := db.Get().DB().Ping()
		assert.NoError(t, err)
	} else {
		db.InitializeTestSqlite3(MigrationFunc)
	}
}
