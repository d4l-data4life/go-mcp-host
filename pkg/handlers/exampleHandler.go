package handlers

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/instrumented"
)

// Define Error messages
const ()

// NewExampleHandler initializes a new handler
func NewExampleHandler() *ExampleHandler {
	return &ExampleHandler{
		Handler: GetHandlerFactory().NewHandler("ExampleHandler"),
	}
}

// ExampleHandler is the handler responsible for Account operations
type ExampleHandler struct {
	*instrumented.Handler
}

// PublicRoutes returns the public routes for the ExampleHandler
func (eh *ExampleHandler) PublicRoutes() *chi.Mux {
	router := chi.NewRouter()
	router.Get(eh.InstrumentChi("/{attribute}", eh.GetExampleByAttribute))
	return router
}

// Routes returns the routes for the ExampleHandler
func (eh *ExampleHandler) InternalRoutes() *chi.Mux {
	router := chi.NewRouter()
	router.Get(eh.InstrumentChi("/{attribute}", eh.GetExampleByAttribute))
	return router
}

// Routes returns the routes for the ExampleHandler
func (eh *ExampleHandler) Routes() *chi.Mux {
	router := chi.NewRouter()
	router.Get(eh.InstrumentChi("/{attribute}", eh.GetExampleByAttribute))
	return router
}

func (eh *ExampleHandler) GetExampleByAttribute(w http.ResponseWriter, r *http.Request) {
	attribute := chi.URLParam(r, "attribute")
	example, err := models.GetExampleByAttribute(attribute)
	if err != nil {
		WriteHTTPErrorCode(w, err, http.StatusNotFound)
	}
	render.JSON(w, r, example)
}
