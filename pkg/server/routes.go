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
	xsrfHandler := middlewares.NewXSRFHandler(viper.GetString("XSRF_SECRET"), viper.GetString("XSRF_HEADER"), handlerFactory)
	authMiddleware := middlewares.NewAuth(viper.GetString("SERVICE_SECRET"), config.PublicKey, handlerFactory)
	xsrfMiddleware := middlewares.NewXSRF(viper.GetString("XSRF_SECRET"), viper.GetString("XSRF_HEADER"), handlerFactory)

	// no auth
	mux.Mount("/checks", ch.Routes())
	mux.Mount("/metrics", promhttp.Handler())

	mux.
		With(middlewares.Trace, logging.Logger().HTTPMiddleware()).
		Route(config.APIPrefixV1, func(r chi.Router) {
			// Auth: none
			r.Mount("/example", exampleHandler.PublicRoutes())

			// Auth: JWT
			r.With(authMiddleware.JWT).
				Mount("/xsrf", xsrfHandler.Routes())

			// Auth: service secret
			r.With(authMiddleware.ServiceSecret).
				Mount("/internal/example", exampleHandler.InternalRoutes())

			// Auth: JWT + XSRF Protection
			r.With(authMiddleware.JWT, xsrfMiddleware.XSRF).
				Mount("/settings", exampleHandler.Routes())
		})

	// Displays all API paths in when debug enabled
	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		route = strings.ReplaceAll(route, "/*/", "/")
		logging.LogDebugf("%s %s\n", method, route)
		return nil
	}
	if err := chi.Walk(mux, walkFunc); err != nil {
		logging.LogErrorf(err, "logging error")
	}
}
