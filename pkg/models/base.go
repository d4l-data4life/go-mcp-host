package models

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// define messages to indentify errors
const (
	PGForeignKeyErrorCode      = "23503"
	PGUniqueViolationErrorCode = "23505"
)

func MigrationFunc(conn *gorm.DB) error {
	// use conn.Debug().AutoMigrate(...) to enable debugging
	// Need to create schema here of migration fails
	conn.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS \"%s\"", viper.GetString("DB_SCHEMA")))
	return conn.AutoMigrate(&Example{})
}

// BaseModel defines the basic fields for each other model
type BaseModel struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// GetFKName constructs the name of the foreign key given the name of the tables (singular/plural important)
// and automatically includes the schema name
func GetFKName(from, to string) string {
	return fmt.Sprintf("fk_%s_%s_%s", viper.GetString("DB_SCHEMA"), from, to)
}
