package handlers_test

import (
	"os"
	"testing"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
)

// Executed before test runs in this package (fails otherwise)
func TestMain(m *testing.M) {
	config.SetupEnv()
	config.SetupLogger()
	os.Exit(m.Run())
}
