package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/models"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	db        *gorm.DB
	jwtSecret []byte
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *gorm.DB, jwtSecret []byte) *AuthHandler {
	return &AuthHandler{
		db:        db,
		jwtSecret: jwtSecret,
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

const (
	// TokenExpirationDuration defines how long a JWT token is valid
	TokenExpirationDuration = 24 * time.Hour
	// TokenIssuer is the issuer claim for generated JWTs
	// #nosec G101 -- This is not a credential, just an identifier
	TokenIssuer = "go-mcp-host"
)

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
	if req.Username == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Username is required"})
		return
	}
	if req.Email == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Email is required"})
		return
	}
	if req.Password == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Password is required"})
		return
	}
	if len(req.Password) < 8 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Password must be at least 8 characters"})
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
	token, err := generateJWT(user.ID, h.jwtSecret)
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
	token, err := generateJWT(user.ID, h.jwtSecret)
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

// generateJWT generates a JWT token for a user with proper claims and signing
func generateJWT(userID uuid.UUID, jwtSecret []byte) (string, error) {
	now := time.Now()

	// Create a new JWT token with standard claims
	token := jwt.New()

	// Set standard claims
	if err := token.Set(jwt.SubjectKey, userID.String()); err != nil {
		return "", errors.Wrap(err, "failed to set subject claim")
	}
	if err := token.Set(jwt.IssuedAtKey, now.Unix()); err != nil {
		return "", errors.Wrap(err, "failed to set issued at claim")
	}
	if err := token.Set(jwt.ExpirationKey, now.Add(TokenExpirationDuration).Unix()); err != nil {
		return "", errors.Wrap(err, "failed to set expiration claim")
	}
	if err := token.Set(jwt.IssuerKey, TokenIssuer); err != nil {
		return "", errors.Wrap(err, "failed to set issuer claim")
	}
	if err := token.Set(jwt.JwtIDKey, uuid.New().String()); err != nil {
		return "", errors.Wrap(err, "failed to set jwt id claim")
	}

	// Sign the token with HS256 algorithm
	signed, err := jwt.Sign(token, jwa.HS256, jwtSecret)
	if err != nil {
		return "", errors.Wrap(err, "failed to sign JWT token")
	}

	return string(signed), nil
}
