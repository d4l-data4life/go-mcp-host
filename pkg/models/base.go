package models

import (
	"time"

	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
)

func MigrationFunc(conn *gorm.DB) error {
	// use conn.Debug().AutoMigrate(...) to enable debugging
	return conn.AutoMigrate(&Example{}).Error
}

// BaseModel defines the basic fields for each other model
type BaseModel struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BeforeCreate will set a UUID rather than numeric ID.
func (base *BaseModel) BeforeCreate(scope *gorm.Scope) error {
	if base.ID == uuid.FromStringOrNil("") {
		uuid := uuid.NewV4()
		return scope.SetColumn("ID", uuid)
	}
	return nil
}
