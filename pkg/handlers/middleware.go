package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/d4l-data4life/go-mcp-host/pkg/auth"
	"github.com/d4l-data4life/go-mcp-host/pkg/models"
)

// ContextKey is the type for context keys
type ContextKey string

const (
	// ContextKeyUserID is the context key for user ID
	ContextKeyUserID ContextKey = "userID"
	// ContextKeyBearerToken is the context key for bearer token
	ContextKeyBearerToken ContextKey = "bearerToken"
)

// AuthMiddleware verifies JWT tokens and adds user ID to context
// If tokenValidator is provided (remote Azure AD keys), it validates using that.
// Otherwise, falls back to simple token parsing (for local development).
func AuthMiddleware(db *gorm.DB, tokenValidator auth.TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get token from Authorization header or query parameter (for WebSocket)
			authHeader := r.Header.Get("Authorization")
			token := ""
			if authHeader != "" {
				// Parse Bearer token
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					token = parts[1]
				}
			}
			if token == "" {
				// Fallback to token query parameter (used for WebSocket where headers can be tricky)
				token = r.URL.Query().Get("token")
			}
			if token == "" {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{"error": "Missing authorization token"})
				return
			}

			var userID uuid.UUID
			var err error

			// If tokenValidator is configured (remote keys), use it
			if tokenValidator != nil {
				userID, err = validateRemoteToken(tokenValidator, token)
				if err != nil {
					render.Status(r, http.StatusUnauthorized)
					render.JSON(w, r, map[string]string{"error": "Invalid or expired token"})
					return
				}

				// Ensure user exists in database (auto-create for Azure AD users)
				if err := models.EnsureUser(userID); err != nil {
					render.Status(r, http.StatusInternalServerError)
					render.JSON(w, r, map[string]string{"error": "Failed to ensure user exists"})
					return
				}
			} else {
				// Fallback to simple token parsing (local development)
				userID, err = parseToken(token)
				if err != nil {
					render.Status(r, http.StatusUnauthorized)
					render.JSON(w, r, map[string]string{"error": "Invalid or expired token"})
					return
				}

				// Validate that the user still exists in the database (only for simple tokens)
				if db != nil {
					var count int64
					if err := db.Table("users").Where("id = ? AND deleted_at IS NULL", userID).Count(&count).Error; err != nil {
						render.Status(r, http.StatusInternalServerError)
						render.JSON(w, r, map[string]string{"error": "Failed to validate user"})
						return
					}
					if count == 0 {
						render.Status(r, http.StatusUnauthorized)
						render.JSON(w, r, map[string]string{
							"error": "User not found - please log in again",
							"code":  "USER_NOT_FOUND",
						})
						return
					}
				}
			}

			// Add user ID and token to context
			ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
			ctx = context.WithValue(ctx, ContextKeyBearerToken, token)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// validateRemoteToken validates a JWT token using remote keys (Azure AD, etc.)
// and extracts the user ID from the token claims
func validateRemoteToken(validator auth.TokenValidator, tokenStr string) (uuid.UUID, error) {
	parsedToken, err := validator.ValidateJWT(tokenStr)
	if err != nil {
		return uuid.Nil, err
	}

	token := *parsedToken // Dereference the pointer

	// Extract user ID from token claims
	// Azure AD tokens typically have 'oid' (object ID) or 'sub' (subject) claim
	var userIDStr string
	if oid, ok := token.Get("oid"); ok {
		userIDStr = oid.(string)
	} else if sub, ok := token.Get("sub"); ok {
		userIDStr = sub.(string)
	} else if email, ok := token.Get("email"); ok {
		// Fallback to email if oid/sub not available
		userIDStr = email.(string)
	} else {
		return uuid.Nil, errors.New("token missing required user identifier claims (oid, sub, or email)")
	}

	// Try to parse as UUID, or hash the string to create a deterministic UUID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		// If not a valid UUID, create a deterministic UUID from the string
		// This ensures consistent user IDs across sessions
		userID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(userIDStr))
	}

	return userID, nil
}

// parseToken parses a simple token and returns the user ID
// In production, use a proper JWT library
func parseToken(token string) (uuid.UUID, error) {
	// Find the first colon to split UUID from timestamp
	// Note: RFC3339 timestamps contain colons, so we can't use strings.Split
	colonIndex := strings.Index(token, ":")
	if colonIndex == -1 || colonIndex == len(token)-1 {
		return uuid.Nil, nil
	}

	userIDStr := token[:colonIndex]
	timestampStr := token[colonIndex+1:]

	// Parse user ID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, err
	}

	// Check expiration
	expiration, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return uuid.Nil, err
	}

	if time.Now().After(expiration) {
		return uuid.Nil, nil
	}

	return userID, nil
}

// GetUserIDFromContext retrieves the user ID from the request context
func GetUserIDFromContext(ctx context.Context) uuid.UUID {
	userID, ok := ctx.Value(ContextKeyUserID).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return userID
}

// GetBearerTokenFromContext retrieves the bearer token from the request context
func GetBearerTokenFromContext(ctx context.Context) string {
	token, ok := ctx.Value(ContextKeyBearerToken).(string)
	if !ok {
		return ""
	}
	return token
}
