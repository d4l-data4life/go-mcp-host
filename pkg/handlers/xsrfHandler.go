package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/spf13/viper"
	"golang.org/x/net/xsrftoken"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc/pkg/instrumented"
)

//NewXSRFHandler initializes a new handler
func NewXSRFHandler() *XSRFHandler {
	return &XSRFHandler{
		Handler:    GetHandlerFactory().NewHandler("XSRFHandler"),
		secret:     viper.GetString("XSRF_SECRET"),
		HeaderName: viper.GetString("XSRF_HEADER"),
	}
}

//XSRFHandler is the handler responsible for Account operations
type XSRFHandler struct {
	*instrumented.Handler
	secret     string
	HeaderName string
}

//Routes returns the routes for the XSRFHandler
func (e *XSRFHandler) Routes() *chi.Mux {
	router := chi.NewRouter()
	router.Get(e.InstrumentChi("/", e.XSRF))
	router.Head(e.InstrumentChi("/", e.XSRF))
	return router
}

// XSRF performs XSRF for given account
func (e *XSRFHandler) XSRF(w http.ResponseWriter, r *http.Request) {
	// Get account id from the request
	accountID, err := ParseRequesterID(w, r)
	if err != nil {
		WriteHTTPErrorCode(w, err, http.StatusBadRequest)
		return
	}

	xsrfToken := xsrftoken.Generate(e.secret, accountID.String(), "")
	w.Header().Set(e.HeaderName, xsrfToken)
}

// NewXSRFMiddleware initializes a new handler
func NewXSRFMiddleware() *XSRFMiddleware {
	return &XSRFMiddleware{
		Handler: GetHandlerFactory().NewHandler("XSRFMiddleware"),
		secret:  viper.GetString("XSRF_SECRET"),
	}
}

// XSRFMiddleware is the handler responsible for XSRF protection
type XSRFMiddleware struct {
	*instrumented.Handler
	secret string
}

// Middleware returns the Middleware HandlerFunc
func (m *XSRFMiddleware) Middleware(next http.Handler) http.Handler {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ignore XSRF for GET, HEAD, OPTIONS methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Test for XSRF token in request header
		token := r.Header.Get(viper.GetString("XSRF_HEADER"))
		if token == "" {
			WriteHTTPErrorCode(w, errors.New("missing XSRF token"), http.StatusForbidden)
			return
		}

		// Get account id from the request
		accountID, err := ParseRequesterID(w, r)
		if err != nil {
			WriteHTTPErrorCode(w, err, http.StatusBadRequest)
			return
		}

		// Validate XSRF token
		if !xsrftoken.Valid(token, m.secret, accountID.String(), "") {
			WriteHTTPErrorCode(w, errors.New("invalid XSRF token"), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
	return m.Instrumenter().Instrument("xsrf", handlerFunc, config.DefaultInstrumentOptions...)
}
