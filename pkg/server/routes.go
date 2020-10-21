package server

import (
	"net/http"
	"strings"

	"github.com/gesundheitscloud/go-svc-template/pkg/config"
	"github.com/gesundheitscloud/go-svc-template/pkg/handlers"
	"github.com/gesundheitscloud/go-svc/pkg/logging"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupRoutes adds all routes that the server should listen to
func SetupRoutes(mux *chi.Mux) {
	ch := handlers.NewChecksHandler()
	auth := handlers.NewAuthMiddleware()
	xsrfHandler := handlers.NewXSRFHandler()
	exampleHandler := handlers.NewExampleHandler()

	mux.Mount("/checks", ch.Routes())
	mux.Mount("/metrics", promhttp.Handler())

	// no auth
	mux.
		With(logging.Logger().HTTPMiddleware()).
		Route(config.APIPrefixV1, func(r chi.Router) {
			// no auth
			r.With(auth.UserAuthMiddleware).
				Mount("/xsrf", xsrfHandler.Routes())

			// Auth: service secret
			r.With(auth.ServiceSecretMiddleware).
				Mount("/internal/example", exampleHandler.InternalRoutes())

			// Auth: JWT
			xsrfMiddleware := handlers.NewXSRFMiddleware()
			r.With(auth.UserAuthMiddleware, xsrfMiddleware.Middleware).
				Mount("/settings", exampleHandler.Routes())
		})

	// Displays all API paths in when debug enabled
	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		route = strings.Replace(route, "/*/", "/", -1)
		logging.LogDebugf("%s %s\n", method, route)
		return nil
	}
	if err := chi.Walk(mux, walkFunc); err != nil {
		logging.LogErrorf(err, "logging error")
	}
}
