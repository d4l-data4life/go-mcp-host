package handlers_test

import (
	"os"
	"testing"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// Executed before test runs in this package (fails otherwise)
func TestMain(m *testing.M) {
	config.SetupEnv()
	config.SetupLogger()
	if err := config.LoadJWTPublicKey(); err != nil {
		logging.LogErrorf(err, "error loading JWT public key")
	}
	os.Exit(m.Run())
}
