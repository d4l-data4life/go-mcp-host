package models

import (
	"errors"
	"fmt"

	"github.com/jackc/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/gesundheitscloud/go-svc/pkg/db2"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// define error messages
var (
	ErrExampleGet                = errors.New("failed getting example")
	ErrExampleUpsert             = errors.New("failed upserting example")
	ErrExampleNotFound           = errors.New("failed finding example")
	ErrExampleDuplicateAttribute = errors.New("duplicate attribute")
)

// define messages to identify errors
const (
	ErrMsgExampleUniqueAttribute = "UNIQUE constraint failed: examples.attribute"
)

// Example model
type Example struct {
	BaseModelWithoutID
	Name       string         `json:"name" gorm:"primaryKey;type:varchar(100)"`
	Attribute  string         `json:"attribute" gorm:"unique"`
	Parameters datatypes.JSON `json:"parameters,omitempty"`
}

func (e Example) String() string {
	return fmt.Sprintf("%s - %s", e.Name, e.Attribute)
}

// Upsert creates/updates an Example object in the Database
func (e *Example) Upsert() error {
	err := db2.Get().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "attribute", "parameters"}),
	}).Create(e).Error

	if err != nil {
		logging.LogErrorf(err, ErrExampleUpsert.Error())
		// Identifies Postgres uniqueness violation error
		if pgErr, isPGErr := err.(*pgconn.PgError); isPGErr {
			if pgErr.Code == PGUniqueViolationErrorCode {
				return ErrExampleDuplicateAttribute
			}
		}
		if err.Error() == ErrMsgExampleUniqueAttribute {
			return ErrExampleDuplicateAttribute
		}
		return ErrExampleUpsert
	}

	return nil
}

func GetExampleByAttribute(attribute string) (Example, error) {
	example := &Example{}
	err := db2.Get().First(example, Example{Attribute: attribute}).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return *example, ErrExampleNotFound
		}
		logging.LogErrorf(err, fmt.Sprintf("Failed getting example for attribute %s", attribute))
		return *example, ErrExampleGet
	}

	return *example, nil
}
