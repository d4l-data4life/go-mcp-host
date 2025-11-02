package handlers

import (
	"context"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// serviceAuthLogger implements the logger interface required by go-svc's ServiceSecretAuthenticator
type serviceAuthLogger struct{}

// NewServiceAuthLogger creates a new service auth logger
func NewServiceAuthLogger() *serviceAuthLogger {
	return &serviceAuthLogger{}
}

// ErrGeneric logs an error generically
func (l *serviceAuthLogger) ErrGeneric(ctx context.Context, err error) error {
	logging.LogErrorf(err, "Service authentication error")
	return err
}
