package handlers

import (
	"context"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// ServiceAuthLogger implements the logger interface required by go-svc's ServiceSecretAuthenticator
type ServiceAuthLogger struct{}

// NewServiceAuthLogger creates a new service auth logger
func NewServiceAuthLogger() *ServiceAuthLogger {
	return &ServiceAuthLogger{}
}

// ErrGeneric logs an error generically
func (l *ServiceAuthLogger) ErrGeneric(ctx context.Context, err error) error {
	logging.LogErrorf(err, "Service authentication error")
	return err
}
