package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/google/uuid"
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
func AuthMiddleware(jwtKey []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{"error": "Missing authorization header"})
				return
			}

			// Parse Bearer token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{"error": "Invalid authorization header format"})
				return
			}

			token := parts[1]

			// Parse token (simple implementation - use proper JWT in production)
			userID, err := parseToken(token)
			if err != nil {
				render.Status(r, http.StatusUnauthorized)
				render.JSON(w, r, map[string]string{"error": "Invalid or expired token"})
				return
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
	parts := strings.Split(token, ":")
	if len(parts) != 2 {
		return uuid.Nil, nil
	}

	// Parse user ID
	userID, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, err
	}

	// Check expiration
	expiration, err := time.Parse(time.RFC3339, parts[1])
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

