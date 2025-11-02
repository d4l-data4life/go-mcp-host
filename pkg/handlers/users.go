package handlers

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/models"
	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// UsersHandler handles user management endpoints
type UsersHandler struct {
	db *gorm.DB
}

// NewUsersHandler creates a new users handler
func NewUsersHandler(db *gorm.DB) *UsersHandler {
	return &UsersHandler{
		db: db,
	}
}

// Routes returns user management routes
func (h *UsersHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.ListUsers)
	r.Delete("/{id}", h.DeleteUser)

	return r
}

// ListUsers returns all users in the system
func (h *UsersHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	var users []models.User

	// Query all users (including soft-deleted ones for admin purposes)
	// Use Unscoped() to include soft-deleted users
	if err := h.db.Unscoped().Find(&users).Error; err != nil {
		logging.LogErrorf(err, "Failed to list users")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to list users"})
		return
	}

	// Convert to public format (remove sensitive data)
	publicUsers := make([]models.PublicUser, len(users))
	for i, user := range users {
		publicUsers[i] = user.ToPublic()
	}

	render.JSON(w, r, publicUsers)
}

// DeleteUser deletes a user by ID
func (h *UsersHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	// Parse user ID from URL
	idStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid user ID"})
		return
	}

	// Check if user exists
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]string{"error": "User not found"})
			return
		}
		logging.LogErrorf(err, "Failed to find user")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to find user"})
		return
	}

	// Delete user (soft delete by default)
	// This will cascade delete conversations and messages
	if err := h.db.Delete(&user).Error; err != nil {
		logging.LogErrorf(err, "Failed to delete user")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to delete user"})
		return
	}

	logging.LogDebugf("User deleted successfully: %s", userID)
	w.WriteHeader(http.StatusNoContent)
}
