package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/d4l-data4life/go-mcp-host/pkg/handlers"
	"github.com/d4l-data4life/go-mcp-host/pkg/models"
	"github.com/d4l-data4life/go-svc/pkg/db"
)

func TestUsersHandler_ListUsers(t *testing.T) {
	models.InitializeTestDB(t)
	defer db.Close()
	handler := handlers.NewUsersHandler(db.Get())

	// Create test users
	user1 := models.User{
		ID: uuid.New(),
	}
	username1 := "testuser1"
	email1 := "test1@example.com"
	user1.Username = &username1
	user1.Email = &email1
	db.Get().Create(&user1)

	user2 := models.User{
		ID: uuid.New(),
	}
	username2 := "testuser2"
	email2 := "test2@example.com"
	user2.Username = &username2
	user2.Email = &email2
	db.Get().Create(&user2)

	// Create a request
	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()

	// Call handler
	handler.ListUsers(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var users []models.PublicUser
	err := json.Unmarshal(w.Body.Bytes(), &users)
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUsersHandler_DeleteUser(t *testing.T) {
	models.InitializeTestDB(t)
	defer db.Close()
	handler := handlers.NewUsersHandler(db.Get())

	// Create test user
	user := models.User{
		ID: uuid.New(),
	}
	username := "testuser"
	email := "test@example.com"
	user.Username = &username
	user.Email = &email
	db.Get().Create(&user)

	// Create a request
	req := httptest.NewRequest("DELETE", "/api/users/"+user.ID.String(), nil)
	w := httptest.NewRecorder()

	// Add chi URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", user.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Call handler
	handler.DeleteUser(w, req)

	// Check response
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify user is soft deleted
	var deletedUser models.User
	err := db.Get().Unscoped().First(&deletedUser, user.ID).Error
	assert.NoError(t, err)
	assert.NotNil(t, deletedUser.DeletedAt)
}

func TestUsersHandler_DeleteUser_NotFound(t *testing.T) {
	models.InitializeTestDB(t)
	defer db.Close()
	handler := handlers.NewUsersHandler(db.Get())

	// Create a request with non-existent user ID
	nonExistentID := uuid.New()
	req := httptest.NewRequest("DELETE", "/api/users/"+nonExistentID.String(), nil)
	w := httptest.NewRecorder()

	// Add chi URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", nonExistentID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Call handler
	handler.DeleteUser(w, req)

	// Check response
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUsersHandler_DeleteUser_InvalidID(t *testing.T) {
	models.InitializeTestDB(t)
	defer db.Close()
	handler := handlers.NewUsersHandler(db.Get())

	// Create a request with invalid user ID
	req := httptest.NewRequest("DELETE", "/api/users/invalid-uuid", nil)
	w := httptest.NewRecorder()

	// Add chi URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Call handler
	handler.DeleteUser(w, req)

	// Check response
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

