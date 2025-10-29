package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/weese/go-mcp-host/pkg/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	db     *gorm.DB
	jwtKey []byte
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *gorm.DB, jwtKey []byte) *AuthHandler {
	return &AuthHandler{
		db:     db,
		jwtKey: jwtKey,
	}
}

// Routes returns auth routes
func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Get("/me", h.GetCurrentUser)

	return r
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	User  models.PublicUser `json:"user"`
	Token string            `json:"token"`
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request body"})
		return
	}

	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Username, email, and password are required"})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := h.db.Where("(username = ? AND username IS NOT NULL) OR (email = ? AND email IS NOT NULL)", req.Username, req.Email).First(&existingUser).Error; err == nil {
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]string{"error": "Username or email already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logging.LogErrorf(err, "Failed to hash password")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to create user"})
		return
	}

	// Create user
	hashedPasswordStr := string(hashedPassword)
	user := models.User{
		ID:           uuid.New(),
		Username:     &req.Username,
		Email:        &req.Email,
		PasswordHash: &hashedPasswordStr,
	}

	if err := h.db.Create(&user).Error; err != nil {
		logging.LogErrorf(err, "Failed to create user")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to create user"})
		return
	}

	// Generate JWT token
	token, err := generateJWT(user.ID, h.jwtKey)
	if err != nil {
		logging.LogErrorf(err, "Failed to generate JWT")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to generate token"})
		return
	}

	username := ""
	if user.Username != nil {
		username = *user.Username
	}
	logging.LogDebugf("User registered: %s", username)

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, AuthResponse{
		User:  user.ToPublic(),
		Token: token,
	})
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request body"})
		return
	}

	// Find user
	var user models.User
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "Invalid username or password"})
		return
	}

	// Verify password
	if user.PasswordHash == nil {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "Invalid username or password"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "Invalid username or password"})
		return
	}

	// Generate JWT token
	token, err := generateJWT(user.ID, h.jwtKey)
	if err != nil {
		logging.LogErrorf(err, "Failed to generate JWT")
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": "Failed to generate token"})
		return
	}

	username := ""
	if user.Username != nil {
		username = *user.Username
	}
	logging.LogDebugf("User logged in: %s", username)

	render.JSON(w, r, AuthResponse{
		User:  user.ToPublic(),
		Token: token,
	})
}

// GetCurrentUser returns the current authenticated user
func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	if userID == uuid.Nil {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]string{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": "User not found"})
		return
	}

	render.JSON(w, r, user.ToPublic())
}

// generateJWT generates a JWT token for a user
func generateJWT(userID uuid.UUID, jwtKey []byte) (string, error) {
	// For now, we'll create a simple token
	// In production, use a proper JWT library like github.com/golang-jwt/jwt
	token := userID.String() + ":" + time.Now().Add(24*time.Hour).Format(time.RFC3339)
	return token, nil
}
