package config

import (
	"fmt"
	"os"
	"runtime"

	"github.com/go-chi/cors"
	"github.com/spf13/viper"

	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// Build information. Populated at build-time.
var (
	Name      = "go-svc-template"
	Version   string
	Branch    string
	Commit    string
	BuildUser string
	GoVersion = runtime.Version()
)

const (
	// EnvPrefix is a prefix to all ENV variables used in this app
	EnvPrefix = "GO_SVC_TEMPLATE"
	// APIPrefixV1 URL prefix in API version 1
	APIPrefixV1 = "/api/v1"

	// ##### GENERAL VARIABLES
	// Debug is a flag used to display debug messages
	Debug = false
	// DebugCORS is a flag used to display CORS debug messages
	DebugCORS = false
	// HumanReadableLogs set to true disables JSON formatting of logging
	HumanReadableLogs = false
	// DefaultPort default port the service is served on
	DefaultPort = "9000"
	// DefaultCorsHosts default cors horst for local development
	DefaultCorsHosts = "https://localhost:3000 http://localhost:3456"

	// ##### DATABASE VARIABLES

	// MigrationVersion determines which migration should be executed
	MigrationVersion  = 1
	DefaultDBHost     = "localhost"
	DefaultDBPort     = "6000"
	DefaultDBName     = "go-svc-template"
	DefaultDBSchema   = "public"
	DefaultDBUser     = "go-svc-template"
	DefaultDBPassword = "postgres"
	DefaultDBSSLMode  = "verify-full"

	// ##### AUTHENTICATION VARIABLES

	// DefaultAuthHeaderName defines the name of the auth header
	DefaultAuthHeaderName = "Authorization"
	// DefaultServiceSecret is a secret used to authenticate requests from other services
	DefaultServiceSecret = ""
)

// ErrorMessage defines the type for the errors channel
type ErrorMessage struct {
	Message string
	Err     error
}

func bindEnvVariable(name string, fallback interface{}) {
	if fallback != "" {
		viper.SetDefault(name, fallback)
	}
	err := viper.BindEnv(name)
	if err != nil {
		// cannot use logging.LogError due to import cycle
		fmt.Printf("Error binding Env Variable: %v", err)
	}
}

// SetupEnv configures app to read ENV variables
func SetupEnv() {
	viper.SetEnvPrefix(EnvPrefix)
	// General
	bindEnvVariable("DEBUG", Debug)
	bindEnvVariable("HUMAN_READABLE_LOGS", HumanReadableLogs)
	bindEnvVariable("DEBUG_CORS", DebugCORS)
	bindEnvVariable("PORT", DefaultPort)
	bindEnvVariable("CORS_HOSTS", DefaultCorsHosts)
	bindEnvVariable("HTTP_MAX_PARALLEL_REQUESTS", 8)
	bindEnvVariable("HTTP_REQUEST_TIMEOUT", "60s")
	// Database
	bindEnvVariable("DB_HOST", DefaultDBHost)
	bindEnvVariable("DB_PORT", DefaultDBPort)
	bindEnvVariable("DB_NAME", DefaultDBName)
	bindEnvVariable("DB_SCHEMA", DefaultDBSchema)
	bindEnvVariable("DB_USER", DefaultDBUser)
	bindEnvVariable("DB_PASS", DefaultDBPassword)
	bindEnvVariable("DB_SSL_MODE", DefaultDBSSLMode)
	bindEnvVariable("DB_SSL_ROOT_CERT_PATH", "/root.ca.pem")
	// Authentication
	bindEnvVariable("AUTH_HEADER_NAME", DefaultAuthHeaderName)
	bindEnvVariable("SERVICE_SECRET", DefaultServiceSecret)
}

// SetupLogger configures the logger with environment preferences
func SetupLogger() {
	logging.LoggerConfig(
		logging.ServiceName("go-svc-template"),
		logging.ServiceVersion(Version),
		logging.Hostname(os.Getenv("HOSTNAME")),
		logging.PodName(os.Getenv("POD_NAME")),
		logging.Environment(os.Getenv("ENVIRONMENT")),
		logging.Debug(viper.GetBool("DEBUG")),
		logging.HumanReadable(viper.GetBool("HUMAN_READABLE_LOGS")),
	)
}

// CorsConfig stores default configuration for CORS middleware
func CorsConfig(corsHosts []string) cors.Options {
	return cors.Options{
		AllowedOrigins:   corsHosts,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-User-Language"},
		ExposedHeaders:   []string{"Link", "X-CSRF-Token"},
		AllowCredentials: true, // header "Access-Control-Allow-Credentials" is not present if this is set to false
		MaxAge:           300,  // Maximum value not ignored by any of major browsers,
		Debug:            viper.GetBool("DEBUG_CORS"),
	}
}
