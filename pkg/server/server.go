package server

import (
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
)

// Server wraps a chi router (chi.Mux)
type Server struct {
	name string
	cors *cors.Cors
	mux  *chi.Mux

	maxParallelProcesses int
	timeout              time.Duration
}

func (s *Server) configMux() *chi.Mux {
	s.mux.Use(
		render.SetContentType(render.ContentTypeJSON), // Set content-Type headers as application/json
		s.cors.Handler, // Set Access-Control-Allow-Origin header
		middleware.RequestID,
		middleware.Compress(5), // Compress results, mostly gzipping assets and json
		middleware.Recoverer,   // Recover from panics without crashing server
		middleware.StripSlashes,
		middleware.RealIP,
		middleware.Timeout(s.timeout),
		middleware.Throttle(s.maxParallelProcesses),
	)
	return s.mux
}

// NewServer creates a router with routes setup
func NewServer(name string,
	cors *cors.Cors,
	maxParallelProcesses int,
	timeout time.Duration,
) *Server {
	s := &Server{
		name:                 name,
		cors:                 cors,
		maxParallelProcesses: maxParallelProcesses,
		timeout:              timeout,
	}
	s.mux = chi.NewRouter()
	s.configMux()
	return s
}

// Mux returns the chi router
func (s *Server) Mux() *chi.Mux {
	return s.mux
}
