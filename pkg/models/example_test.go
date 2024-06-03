package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/gesundheitscloud/go-svc-template/internal/testutils"
	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/db"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

func TestExample_Upsert(t *testing.T) {
	example := InitDBWithTestExample(t)
	tests := []struct {
		name        string
		exampleName string
		attribute   string
		err         error
	}{
		{"Create", "test1", "random", nil},
		{"Update", "test1", "random2", nil},
		{"Duplicate attribute", "test2", example.Attribute, models.ErrExampleDuplicateAttribute},
	}
	defer db.Close()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			example := CreateExample(tt.exampleName, tt.attribute)
			err := example.Upsert()
			logging.LogErrorf(err, "error")
			if tt.err == nil {
				assert.NoError(t, err)
				_, err := models.GetExampleByAttribute(tt.attribute)
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func TestGetExampleByAttribute(t *testing.T) {
	example := InitDBWithTestExample(t)
	defer db.Close()
	tests := []struct {
		name string
		want models.Example
		err  error
	}{
		{"activated account", example, nil},
		{"not found", CreateExample("something", "something"), models.ErrExampleNotFound},
	}
	defer db.Close()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := models.GetExampleByAttribute(tt.want.Attribute)
			if tt.err == nil {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.String(), got.String())
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.err)
			}
		})
	}
}
