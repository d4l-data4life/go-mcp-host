package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	uuid "github.com/satori/go.uuid"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc-template/pkg/handlers"
)

func TestParseRequesterID(t *testing.T) {
	id := uuid.NewV4()
	tests := []struct {
		name      string
		accountID string
		want      uuid.UUID
		wantErr   bool
	}{
		{"Success", id.String(), id, false},
		{"Failure", "not a uuid", id, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			request, _ := http.NewRequest("method", "url", strings.NewReader(""))
			request = request.WithContext(context.WithValue(request.Context(), handlers.RequesterContext, tt.accountID))
			writer := httptest.NewRecorder()
			got, err := handlers.ParseRequesterID(writer, request)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRequesterID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				assert.Equal(t, got.String(), tt.accountID, "UUIDs should match")
			}
		})
	}
}

func TestAuthMiddlewareServiceRequest(t *testing.T) {
	config.SetupEnv()
	validAuthHeader := viper.GetString("SERVICE_SECRET")
	authHeaderName := viper.GetString("AUTH_HEADER_NAME")
	tests := []struct {
		name              string
		URL               string
		AuthHeaderContent string
		expectedStatus    int
	}{
		{"valid Service Auth", "/service", validAuthHeader, http.StatusOK},
		{"invalid Service Auth", "/service", "random", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			nextRequest, _ := http.NewRequest(http.MethodGet, tt.URL, nil)
			nextRequest.Header.Add(authHeaderName, tt.AuthHeaderContent)
			nextResponse := httptest.NewRecorder()

			// Fire another backend request
			dummyMiddleware := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			authMiddleware := handlers.NewAuthMiddleware()
			authMiddleware.ServiceSecretMiddleware(dummyMiddleware).ServeHTTP(nextResponse, nextRequest)
			assert.Equal(t, tt.expectedStatus, nextResponse.Code)
		})
	}
}
