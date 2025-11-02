package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/d4l-data4life/go-svc/pkg/db"
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Username     *string        `gorm:"size:255;uniqueIndex"                           json:"username,omitempty"`
	Email        *string        `gorm:"size:255;uniqueIndex"                           json:"email,omitempty"`
	PasswordHash *string        `gorm:"size:255"                                       json:"-"` // Never expose password hash in JSON
	CreatedAt    time.Time      `                                                      json:"createdAt"`
	UpdatedAt    time.Time      `                                                      json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index"                                          json:"deletedAt,omitempty"`

	// Associations
	Conversations []Conversation `gorm:"foreignKey:UserID" json:"conversations,omitempty"`
}

// TableName specifies the table name for User model
func (User) TableName() string {
	return "users"
}

// BeforeCreate hook to ensure ID is set
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// PublicUser represents user data safe for public consumption
type PublicUser struct {
	ID        uuid.UUID `json:"id"`
	Username  *string   `json:"username,omitempty"`
	Email     *string   `json:"email,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// ToPublic converts User to PublicUser (removes sensitive data)
func (u *User) ToPublic() PublicUser {
	return PublicUser{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}

// EnsureUser makes sure the user with the given ID exists
func EnsureUser(userID uuid.UUID) error {
	u := &User{ID: userID}
	return db.Get().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoNothing: true,
	}).Create(u).Error
}
