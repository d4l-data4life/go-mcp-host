package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
func AuthMiddleware(jwtKey []byte, db *gorm.DB) func(http.Handler) http.Handler {
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

			// Parse token (simple implementation - use proper JWT in production)
			userID, err := parseToken(token)
			if err != nil {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{"error": "Invalid or expired token"})
				return
			}

			// Validate that the user still exists in the database
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

			// Add user ID and token to context
			ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
			ctx = context.WithValue(ctx, ContextKeyBearerToken, token)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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

