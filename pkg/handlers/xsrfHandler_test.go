package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gesundheitscloud/go-svc-template/pkg/handlers"
)

func TestXSRFTokenGeneration(t *testing.T) {
	request, _ := http.NewRequest("method", "url", nil)
	response := httptest.NewRecorder()

	e := handlers.NewXSRFHandler()
	ctx := request.Context()
	ctx = context.WithValue(ctx, handlers.RequesterContext, "123e4567-e89b-12d3-a456-426655440000")
	request = request.WithContext(ctx)
	e.XSRF(response, request)
	assert.Equal(t, http.StatusOK, response.Code)

	assert.NotEmpty(t, response.Header().Get(e.HeaderName))
}
