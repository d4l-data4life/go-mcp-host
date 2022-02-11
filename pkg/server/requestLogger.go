package server

import (
	"net/http"

	"github.com/gesundheitscloud/go-svc/pkg/d4lcontext"
	"github.com/gesundheitscloud/go-svc/pkg/log"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// RequestLogger sets up the middleware to log requests
func RequestLogger() func(http.Handler) http.Handler {
	return logging.Logger().HTTPMiddleware(
		log.WithUserParser(d4lcontext.GetUserIDString),
		log.WithClientIDParser(d4lcontext.GetClientID),
		log.WithCallerIPParser(getCallerIPFromRequest),
		LogObfuscators(),
	)
}

// getCallerIPFromRequest is used by the logger to extract the caller's IP address
func getCallerIPFromRequest(r *http.Request) string {
	return r.RemoteAddr
}

// LogObfuscators returns log obfuscators for use with the http logging middleware
func LogObfuscators() func(*log.HTTPLogger) {
	return log.WithObfuscators()
}
