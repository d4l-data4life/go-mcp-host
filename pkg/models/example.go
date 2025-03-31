package models

import (
	"encoding/json"
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

// Example model
type Example struct {
	BaseModel
	Name       string         `json:"name"                 gorm:"primaryKey"`
	Attribute  string         `json:"attribute"            gorm:"unique"`
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
		logging.LogErrorf(err, "%s", ErrExampleUpsert.Error())
		// Identifies Postgres uniqueness violation error
		if pgErr, isPGErr := err.(*pgconn.PgError); isPGErr {
			if pgErr.Code == PGUniqueViolationErrorCode {
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
		logging.LogErrorf(err, "Failed getting example for attribute %s", attribute)
		return example, ErrExampleGet
	}

	return example, nil
}

// Examples used to show why linter is set to 140 characters
func LongMethodSignatureButJustShortEnough(attributes string, values int, styles string, flavours string, potentials int) (Example, error) {
	example := Example{
		Attribute: attributes,
	}
	params := map[string]any{
		"values":     values,
		"styles":     styles,
		"flavours":   flavours,
		"potentials": potentials,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return example, err
	}
	example.Parameters = paramsJSON

	return example, nil
}

func TooLongMethodNameWithFarTooManyParameters(
	attribute string,
	value int,
	style string,
	flavour string,
	potential int,
	overboard string,
) (Example, error) {
	example := Example{
		Attribute: attribute,
	}
	params := map[string]any{
		"value":     value,
		"style":     style,
		"flavour":   flavour,
		"potential": potential,
		"overboard": overboard,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return example, err
	}
	example.Parameters = paramsJSON

	return example, nil
}
