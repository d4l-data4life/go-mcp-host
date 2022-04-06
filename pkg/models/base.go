package models

import (
	"time"

	"gorm.io/gorm"
)

// define messages to indentify errors
const (
	PGForeignKeyErrorCode      = "23503"
	PGUniqueViolationErrorCode = "23505"
)

func MigrationFunc(conn *gorm.DB) error {
	// use conn.Debug().AutoMigrate(...) to enable debugging
	return conn.AutoMigrate(&Example{})
}

// BaseModel defines the basic fields for each other model
type BaseModel struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
