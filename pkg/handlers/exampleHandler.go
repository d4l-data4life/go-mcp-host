package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"

	"github.com/gesundheitscloud/go-svc-template/pkg/models"
	"github.com/gesundheitscloud/go-svc/pkg/instrumented"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
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
	router.Post(eh.InstrumentChi("/", eh.UpsertExample))
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
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	render.JSON(w, r, example)
}

func (eh *ExampleHandler) UpsertExample(w http.ResponseWriter, r *http.Request) {
	example := models.Example{}
	err := json.NewDecoder(r.Body).Decode(&example)
	if err != nil {
		logging.LogErrorfCtx(r.Context(), err, "Error decoding request payload")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = example.Upsert()
	if err != nil {
		switch {
		case errors.Is(err, models.ErrExampleDuplicateAttribute):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
