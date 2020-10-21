package handlers

import (
	"net/http"

	"github.com/go-chi/chi"

	"github.com/gesundheitscloud/go-svc/pkg/db"
	"github.com/gesundheitscloud/go-svc/pkg/instrumented"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
)

//ChecksHandler is the handler responsible for k8s checks
type ChecksHandler struct {
	*instrumented.Handler
}

//Routes returns the routes for the ChecksHandler
func (e *ChecksHandler) Routes() *chi.Mux {
	router := chi.NewRouter()
	router.Get(e.InstrumentChi("/liveness", e.Liveness))
	router.Get(e.InstrumentChi("/readiness", e.Readiness))
	return router
}

//NewChecksHandler initializes a new handler
func NewChecksHandler() *ChecksHandler {
	return &ChecksHandler{
		Handler: GetHandlerFactory().NewHandler("K8sChecksHandler"),
	}
}

//Liveness is a check that describes if the application has started
func (e *ChecksHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	WriteHTTPCode(w, http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		logging.LogError("Error writing OK to response body", err)
	}
}

//Readiness is a check if application can handle requests
func (e *ChecksHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if err := db.Get().DB().Ping(); err != nil {
		WriteHTTPErrorCode(w, err, http.StatusInternalServerError)
		return
	}

	WriteHTTPCode(w, http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		logging.LogError("Error writing OK to response body", err)
	}
}
