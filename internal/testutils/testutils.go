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

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc-template/pkg/metrics"
	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc-template/pkg/server"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

func CreateExample(name string, attribute string) models.Example {
	example := models.Example{
		Name:      name,
		Attribute: attribute,
	}
	return example
}

// InitDBWithTestExample inits a test db with one registred and one activated account
func InitDBWithTestExample(t *testing.T) (example models.Example) {
	models.InitializeTestDB(t)
	return AddExamplesToDB()
}

// AddTestDataExamplesToDB adds examples test data to the database
func AddTestDataExamplesToDB() (example models.Example) {
	example = CreateExample("test_persistent", "test")
	if err := example.Upsert(); err != nil {
		logging.LogErrorf(err, "Error in test Setup")
	}
	return example
}

func AddExamplesToDB() (example models.Example) {
	example = CreateExample("test", "test")
	if err := example.Upsert(); err != nil {
		logging.LogErrorf(err, "Error in test Setup")
	}
	return example
}

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

	server.SetupRoutes(srv.Mux())
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
