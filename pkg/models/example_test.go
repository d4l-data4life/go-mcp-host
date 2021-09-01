package models_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gesundheitscloud/go-svc-template/internal/testutils"
	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/db2"
)

func TestExample_Upsert(t *testing.T) {
	example := testutils.InitDBWithTestExample(t)
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
	defer db2.Close()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			example := testutils.CreateExample(tt.exampleName, tt.attribute)
			err := example.Upsert()

			if tt.err == nil {
				assert.NoError(t, err, "Upsert() shouldn't return an error")
				_, err := models.GetExampleByAttribute(tt.attribute)
				assert.NoError(t, err, "No error should be returned")
			} else {
				assert.Truef(t, errors.Is(err, tt.err), "Upsert() returns wrong error %v != %v", err, tt.err)
			}
		})
	}
}

func TestGetExampleByAttribute(t *testing.T) {
	example := testutils.InitDBWithTestExample(t)
	tests := []struct {
		name    string
		want    models.Example
		wantErr bool
	}{
		{"activated account", example, false},
		{"not found", testutils.CreateExample("something", "something"), true},
	}
	defer db2.Close()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := models.GetExampleByAttribute(tt.want.Attribute)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetExampleByAttribute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.Equal(t, tt.want.String(), got.String(), "Should match")
			}
		})
	}
}
