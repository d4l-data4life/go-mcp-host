package handlers

import (
	"net/http"

	"github.com/go-chi/chi"

	"github.com/gesundheitscloud/go-svc/pkg/db2"
	"github.com/gesundheitscloud/go-svc/pkg/instrumented"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

// ChecksHandler is the handler responsible for k8s checks
type ChecksHandler struct {
	*instrumented.Handler
}

// Routes returns the routes for the ChecksHandler
func (e *ChecksHandler) Routes() *chi.Mux {
	router := chi.NewRouter()
	router.Get(e.InstrumentChi("/liveness", e.Liveness))
	router.Get(e.InstrumentChi("/readiness", e.Readiness))
	return router
}

// NewChecksHandler initializes a new handler
func NewChecksHandler() *ChecksHandler {
	return &ChecksHandler{
		Handler: GetHandlerFactory().NewHandler("K8sChecksHandler"),
	}
}

// Liveness is a check that describes if the application has started
func (e *ChecksHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	// We use the stricter readiness check also for liveness to make
	// K8s restart the pod if something is wrong with the DB connection.
	e.Readiness(w, r)
}

// Readiness is a check if application can handle requests
func (e *ChecksHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if err := db2.Ping(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		logging.LogErrorfCtx(r.Context(), err, "Error writing OK to response body")
	}
}
