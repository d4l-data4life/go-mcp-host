package server

import (
	"net/http"
	"strings"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc-template/pkg/handlers"
	"github.com/gesundheitscloud/go-svc/pkg/logging"
	"github.com/gesundheitscloud/go-svc/pkg/middlewares"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

// SetupRoutes adds all routes that the server should listen to
func SetupRoutes(mux *chi.Mux) {
	ch := handlers.NewChecksHandler()
	exampleHandler := handlers.NewExampleHandler()

	handlerFactory := handlers.GetHandlerFactory()
	// needs to be adjusted for azure authentication
	authMiddleware := middlewares.NewAuthentication(viper.GetString("SERVICE_SECRET"), handlerFactory)

	// no auth
	mux.Mount("/checks", ch.Routes())
	mux.Mount("/metrics", promhttp.Handler())

	mux.
		With(middlewares.Trace).
		Route(config.APIPrefixV1, func(r chi.Router) {
			// Auth: none
			r.With(RequestLogger()).
				Mount("/public/example", exampleHandler.PublicRoutes())

			// Auth: JWT (use after adjusting)
			r.With(RequestLogger()).
				Mount("/example", exampleHandler.Routes())

			// Auth: service secret
			r.With(authMiddleware.ServiceSecret, RequestLogger()).
				Mount("/internal/example", exampleHandler.InternalRoutes())
		})

	// Displays all API paths when debug enabled
	walkFunc := func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.ReplaceAll(route, "/*/", "/")
		logging.LogDebugf("%s %s\n", method, route)
		return nil
	}
	if err := chi.Walk(mux, walkFunc); err != nil {
		logging.LogErrorf(err, "logging error")
	}
}
