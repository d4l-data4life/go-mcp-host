package models

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gesundheitscloud/go-svc/pkg/db"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// define error messages
var (
	ErrExampleGet                = errors.New("failed getting example")
	ErrExampleUpsert             = errors.New("failed upserting example")
	ErrExampleNotFound           = errors.New("failed finding example")
	ErrExampleDuplicateAttribute = errors.New("duplicate attribute")
)

// define postgres constraints
const (
	UniqueAttribute = "uni_public_examples_attribute"
)

// Example model
type Example struct {
	BaseModel
	Name       string         `json:"name" gorm:"primaryKey"`
	Attribute  string         `json:"attribute" gorm:"unique"`
	Parameters datatypes.JSON `json:"parameters,omitempty"`
}

func (e Example) String() string {
	return fmt.Sprintf("%s - %s", e.Name, e.Attribute)
}

func (Example) UpdateableColumns() []string {
	return []string{"updated_at", "attribute", "parameters"}
}

// Upsert creates/updates an Example object in the Database
func (e *Example) Upsert() error {
	err := db.Get().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns(e.UpdateableColumns()),
	}).Create(e).Error

	if err != nil {
		logging.LogErrorf(err, ErrExampleUpsert.Error())
		// Identifies Postgres uniqueness violation error
		if pgErr, isPGErr := err.(*pgconn.PgError); isPGErr {
			if pgErr.ConstraintName == UniqueAttribute {
				return ErrExampleDuplicateAttribute
			}
		}
		return ErrExampleUpsert
	}

	return nil
}

func GetExampleByAttribute(attribute string) (Example, error) {
	example := Example{}
	err := db.Get().First(&example, Example{Attribute: attribute}).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return example, ErrExampleNotFound
		}
		logging.LogErrorf(err, fmt.Sprintf("Failed getting example for attribute %s", attribute))
		return example, ErrExampleGet
	}

	return example, nil
}
