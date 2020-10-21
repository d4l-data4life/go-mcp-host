package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gesundheitscloud/go-svc-template/internal/testutils"
	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/db"
)

func TestExample_Create(t *testing.T) {
	example := testutils.InitDBWithTestExample(t)
	tests := []struct {
		name      string
		attribute string
		wantErr   bool
	}{
		{"Create complete", "random", false},
		{"Duplicate attribute", example.Attribute, true},
	}
	defer db.Get().Close()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			example := testutils.CreateExample(tt.attribute)
			if err := example.Create(); (err != nil) != tt.wantErr {
				t.Errorf("Example.Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				_, err := models.GetExampleByAttribute(tt.attribute)
				assert.NoError(t, err, "No error should be returned")
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
		{"not found", testutils.CreateExample("something"), true},
	}
	defer db.Get().Close()
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
