package models

import (
	"errors"
	"fmt"

	"github.com/jinzhu/gorm"

	"github.com/gesundheitscloud/go-svc/pkg/db"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// define error messages
var (
	ErrExampleGet         = errors.New("failed getting example")
	ErrExampleCreation    = errors.New("failed creating example")
	ErrExampleNotFound    = errors.New("failed finding example")
	ErrDuplicateAttribute = errors.New("UNIQUE constraint failed: examples.attribute")
)

// Example model
type Example struct {
	BaseModel
	Attribute string `json:"attribute" gorm:"unique"`
}

func (example Example) String() string {
	return fmt.Sprintf("%s - %s", example.ID, example.Attribute)
}

// Create creates Account object in the Database
func (example *Example) Create() error {
	err := db.Get().Create(example).Error

	if example.ID.String() == "" || err != nil {
		logging.LogError("", err)
		if err.Error() == ErrDuplicateAttribute.Error() {
			return ErrDuplicateAttribute
		}
		return ErrExampleCreation
	}

	return nil
}

func GetExampleByAttribute(attribute string) (Example, error) {
	example := &Example{}
	err := db.Get().First(example, Example{Attribute: attribute}).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return *example, ErrExampleNotFound
		}
		logging.LogError(fmt.Sprintf("Failed getting example for attribute %s", attribute), err)
		return *example, ErrExampleGet
	}

	return *example, nil
}
