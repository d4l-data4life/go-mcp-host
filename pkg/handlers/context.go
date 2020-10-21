package handlers

import (
	"context"
	"net/http"
)

type contextKey string

//AddToContext adds a variable to the request context
func AddToContext(name contextKey, value string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), name, value))
			next.ServeHTTP(w, r)
		})
	}
}
