package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/d4l-data4life/go-mcp-host/pkg/handlers"
	"github.com/d4l-data4life/go-mcp-host/pkg/models"

	"github.com/d4l-data4life/go-svc/pkg/db"
)

const (
	livenessURL  = "/checks/liveness"
	readinessURL = "/checks/readiness"
)

func TestRoutesCheck(t *testing.T) {
	router := handlers.NewChecksHandler().Routes()
	assert.NotNil(t, router)
	assert.Len(t, router.Routes(), 2)
}

func TestCheckLiveness(t *testing.T) {
	models.InitializeTestDB(t)
	defer db.Close()
	request, _ := http.NewRequest(http.MethodGet, livenessURL, nil)
	response := httptest.NewRecorder()
	handlers.NewChecksHandler().Liveness(response, request)
	assert.Equal(t, 200, response.Code)
}

func TestCheckReadiness(t *testing.T) {
	models.InitializeTestDB(t)
	defer db.Close()
	request, _ := http.NewRequest(http.MethodGet, readinessURL, nil)
	response := httptest.NewRecorder()
	handlers.NewChecksHandler().Readiness(response, request)
	assert.Equal(t, 200, response.Code)
}

func TestCheckReadinessFailure(t *testing.T) {
	// Open and Close DB connection to simulate broken connection
	models.InitializeTestDB(t)
	db.Close()
	request, _ := http.NewRequest(http.MethodGet, readinessURL, nil)
	response := httptest.NewRecorder()
	handlers.NewChecksHandler().Readiness(response, request)
	assert.Equal(t, 500, response.Code)
}
