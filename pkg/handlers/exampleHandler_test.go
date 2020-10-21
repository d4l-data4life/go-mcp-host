package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"

	"github.com/gesundheitscloud/go-svc-template/internal/testutils"
	"github.com/gesundheitscloud/go-svc-template/pkg/handlers"
	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/db"
)

func TestRoutesConsent(t *testing.T) {
	router := handlers.NewExampleHandler().Routes()
	assert.NotNil(t, router, "should return a valid router")
}

func TestExampleHandler_GetExampleByAttribute(t *testing.T) {
	example := testutils.InitDBWithTestExample(t)
	tests := []struct {
		name            string
		attribute       string
		expectedExample models.Example
		statusCode      int
	}{
		{"success", example.Attribute, example, http.StatusOK},
		{"not found", "random", models.Example{}, http.StatusNotFound},
	}
	defer db.Get().Close()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := &handlers.ExampleHandler{}
			request, _ := http.NewRequest("method", "url", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("attribute", tt.attribute)
			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))
			writer := httptest.NewRecorder()
			e.GetExampleByAttribute(writer, request)
			assert.Equal(t, tt.statusCode, writer.Code)
			if tt.statusCode == http.StatusOK {
				result := models.Example{}
				err := json.NewDecoder(writer.Body).Decode(&result)
				assert.NoError(t, err, "should not error on decode")
				assert.Equal(t, tt.expectedExample.String(), result.String(), "should return expected example")
			}
		})
	}
}
