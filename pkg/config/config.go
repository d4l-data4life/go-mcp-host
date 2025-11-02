package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// Build information. Populated at build-time.
var (
	Name      = "go-mcp-host"
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
	DefaultPort = "8080"
	// DefaultCorsHosts default cors horst for local development
	DefaultCorsHosts = "http://localhost:3334 https://localhost:3000 http://localhost:3456"

	// ##### DATABASE VARIABLES

	// MigrationVersion determines which migration should be executed
	MigrationVersion  = 1
	DefaultDBHost     = "localhost"
	DefaultDBPort     = "6000"
	DefaultDBName     = "go-mcp-host"
	DefaultDBSchema   = "public"
	DefaultDBUser     = "go-mcp-host"
	DefaultDBPassword = "postgres"
	DefaultDBSSLMode  = "verify-full"

	// ##### AUTHENTICATION VARIABLES
	DefaultJWTSecret     = "" // Base64-encoded JWT secret
	DefaultRemoteKeysURL = "" // Empty by default; set to use Azure AD or similar
)

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

func loadDotEnv() {
	path := ".env"
	absPath, err := filepath.Abs(path)
	if err != nil {
		logging.LogErrorf(err, "failed calculating absolute path")
	}
	// for binary in docker dont load .env because its handled via helm
	if !strings.Contains(absPath, Name) {
		return
	}
	// recursively go up a folder until repository root is found
	for !strings.HasSuffix(absPath, fmt.Sprintf("%s/.env", Name)) {
		path = "../" + path
		absPath, err = filepath.Abs(path)
		if err != nil {
			logging.LogErrorf(err, "failed calculating absolute path")
			return
		}
	}
	err = godotenv.Load(absPath)
	if err != nil {
		logging.LogWarningf(err, "failed loading .env file at %s", absPath)
	}
}

func loadConfigFile() {
	// Set config file name and type
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Add config paths to search
	viper.AddConfigPath(".")      // Current directory
	viper.AddConfigPath("./")     // Current directory
	viper.AddConfigPath("../")    // Parent directory
	viper.AddConfigPath("../../") // Two levels up

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; using defaults and environment variables
			logging.LogDebugf("No config.yaml found, using environment variables and defaults")
		} else {
			// Config file was found but another error was produced
			logging.LogWarningf(err, "Error reading config file")
		}
	} else {
		logging.LogDebugf("Loaded configuration from: %s", viper.ConfigFileUsed())
	}
}

// SetupEnv configures app to read ENV variables
func SetupEnv() {
	loadDotEnv()
	loadConfigFile()
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
	bindEnvVariable("SERVICE_SECRET", "")
	bindEnvVariable("JWT_SECRET", DefaultJWTSecret)
	bindEnvVariable("JWT_EXPIRATION_HOURS", 24)
	bindEnvVariable("REMOTE_KEYS_URL", DefaultRemoteKeysURL)
	// MCP and Agent configuration
	SetupMCPEnv()
}

// SetupLogger configures the logger with environment preferences
func SetupLogger() {
	logging.LoggerConfig(
		logging.ServiceName("go-mcp-host"),
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
