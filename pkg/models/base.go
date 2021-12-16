package models

import (
	"time"

	uuid "github.com/gofrs/uuid"
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

// BaseModelWithoutID defines the basic fields for each other model
type BaseModelWithoutID struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BaseModel defines the basic fields for each other model
type BaseModel struct {
	ID uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;"`
	BaseModelWithoutID
}

// BeforeCreate will set a UUID rather than numeric ID.
func (base *BaseModel) BeforeCreate(scope *gorm.DB) error {
	if base.ID == uuid.Nil {
		uuid, err := uuid.NewV4()
		if err != nil {
			return err
		}
		base.ID = uuid
	}
	return nil
}
