package testutils

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/cors"
	"gorm.io/datatypes"

	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/metrics"
	"github.com/d4l-data4life/go-mcp-host/pkg/models"
	"github.com/d4l-data4life/go-mcp-host/pkg/server"
	"github.com/d4l-data4life/go-svc/pkg/logging"
)

// GetRequestPayload converts a given object into a reader of that obect as json payload
func GetRequestPayload(payload interface{}) io.Reader {
	bytes, _ := json.Marshal(payload)
	return strings.NewReader(string(bytes))
}

// GetTestMockServer creates the mocked server for tests
func GetTestMockServer(t *testing.T) *server.Server {
	models.InitializeTestDB(t)
	corsOptions := config.CorsConfig([]string{"localhost"})
	srv := server.NewServer("TEST_SERVER", cors.New(corsOptions), 1, 10*time.Second)

	// Use a test JWT secret for testing
	testJWTSecret := []byte("test-secret-key-for-testing-only-32bytes!!")
	server.SetupRoutes(context.Background(), srv.Mux(), nil, testJWTSecret) // nil tokenValidator for tests
	metrics.AddBuildInfoMetric()
	return srv
}

// RunningTime starts measuring runtime - usage defer Track(RunningTime("label"))
func RunningTime(s string) (string, time.Time) {
	log.Println("Start:	", s)
	return s, time.Now()
}

// Track finishes measuring runtime and prints result - usage defer Track(RunningTime("label"))
func Track(s string, startTime time.Time) {
	endTime := time.Now()
	log.Println("End:	", s, "took", endTime.Sub(startTime))
}

type stringer interface {
	String() string
}

// StringSort sorts slices of elements by string representation method for deterministic tests
func StringSort[T stringer](slice []T) {
	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].String() < slice[j].String()
	})
}

func MustJSON[T any](object T) datatypes.JSON {
	bytes, err := json.Marshal(object)
	if err != nil {
		logging.LogErrorfCtx(context.Background(), err, "failed marshalling to JSON")
		return nil
	}
	return bytes
}

func Pointerfy[T any](thing T) *T {
	return &thing
}
