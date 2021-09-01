package server_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gesundheitscloud/go-svc-template/internal/testutils"
	"github.com/gesundheitscloud/go-svc/pkg/db2"
)

func TestEndpointProtection(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		url       string
		protected bool
	}{
		{"Liveness", http.MethodGet, "/checks/liveness", false},
		{"Readiness", http.MethodGet, "/checks/readiness", false},
		{"Metrics", http.MethodGet, "/metrics", false},
	}

	server := testutils.GetTestMockServer(t)
	defer db2.Close()

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.url, strings.NewReader(""))
			writer := httptest.NewRecorder()
			server.Mux().ServeHTTP(writer, request)
			assert.Equal(t, test.protected, writer.Code == http.StatusUnauthorized)
		})
	}
}

func TestMetrics(t *testing.T) {
	tests := []struct {
		name        string
		metric      string
		value       int
		metricExist bool
		valueMatch  bool
	}{
		{"Golang metrics should exist", "go_memstats_alloc_bytes_total", -1, true, false},
		{"Golang metrics should exist", "go_info", 1, true, true},
		{"go-svc-template info metric should exist", "build_info", 1, true, true},
	}

	server := testutils.GetTestMockServer(t)
	defer db2.Close()

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/metrics", strings.NewReader(""))
			writer := httptest.NewRecorder()
			server.Mux().ServeHTTP(writer, request)

			resp := writer.Result()
			body, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()

			assert.Equal(t, test.metricExist, strings.Contains(string(body), test.metric),
				fmt.Sprintf("Text %s should contain metric '%s'", string(body), test.metric))

			// regexp allows to ignore metric labels
			metricValueRegexp := fmt.Sprintf(`%s(\{.*\})? %d`, test.metric, test.value)
			matched, err := regexp.Match(metricValueRegexp, body)
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, test.valueMatch, matched,
				fmt.Sprintf("Text %s should contain metric '%s' with value '%d'", string(body), test.metric, test.value))
		})
	}
}

func TestCors(t *testing.T) {
	tests := []struct {
		name                  string
		reply                 *httptest.ResponseRecorder
		request               *http.Request
		requestHeader         string // one header to include in request (cannot use maps here)
		requestHeaderContent  string // header value
		expectHeaders         bool   // whether expectedHeader should be present in reply
		expectedHeader        string
		expectedHeaderContent string
	}{
		{
			name:                  "Access-Control-Allow-Origin header should be present",
			reply:                 httptest.NewRecorder(),
			request:               httptest.NewRequest("GET", "/checks/liveness", nil),
			requestHeader:         "Origin",
			requestHeaderContent:  "localhost",
			expectHeaders:         true,
			expectedHeader:        "Access-Control-Allow-Origin",
			expectedHeaderContent: "localhost",
		},
		{
			name:                  "Access-Control-Expose-Headers header should be present",
			reply:                 httptest.NewRecorder(),
			request:               httptest.NewRequest("GET", "/checks/liveness", nil),
			requestHeader:         "Origin",
			requestHeaderContent:  "localhost",
			expectHeaders:         true,
			expectedHeader:        "Access-Control-Expose-Headers",
			expectedHeaderContent: "Link, X-Csrf-Token",
		},
		{
			name:                  "Access-Control-Allow-Credentials header should be present",
			reply:                 httptest.NewRecorder(),
			request:               httptest.NewRequest("GET", "/checks/liveness", nil),
			requestHeader:         "Origin",
			requestHeaderContent:  "localhost",
			expectHeaders:         true,
			expectedHeader:        "Access-Control-Allow-Credentials",
			expectedHeaderContent: "true",
		},
		{
			name:                  "Origin matches not",
			reply:                 httptest.NewRecorder(),
			request:               httptest.NewRequest("GET", "/checks/liveness", nil),
			requestHeader:         "Origin",
			requestHeaderContent:  "http://www.data4life.care",
			expectHeaders:         false,
			expectedHeader:        "Access-Control-Allow-Origin",
			expectedHeaderContent: "localhost",
		},
	}

	server := testutils.GetTestMockServer(t)
	defer db2.Close()

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			test.request.Header.Set(test.requestHeader, test.requestHeaderContent)

			server.Mux().ServeHTTP(test.reply, test.request)
			if test.expectHeaders {
				assert.Equal(t, test.expectedHeaderContent, test.reply.Header().Get(test.expectedHeader))
			} else {
				assert.Equal(t, "", test.reply.Header().Get(test.expectedHeader))
			}
		})
	}
}
