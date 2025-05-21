package http

import (
	// Go Internal Packages
	"context"
	"net/http"
	"time"

	// Local Packages
	errors "scheduler/errors"
	handlers "scheduler/http/handlers"
	smiddleware "scheduler/http/middlewares"
	resp "scheduler/http/response"
	health "scheduler/services/health"

	// External Packages
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// Server struct follows the alphabet order
type Server struct {
	health    *health.HealthCheckService
	logger    *zap.Logger
	prefix    string
	scheduler *handlers.SchedulerHandler
}

func NewServer(
	prefix string,
	logger *zap.Logger,
	healthCheckService *health.HealthCheckService,
	scheduler *handlers.SchedulerHandler,
) *Server {
	return &Server{
		prefix:    prefix,
		logger:    logger,
		health:    healthCheckService,
		scheduler: scheduler,
	}
}

func (s *Server) Listen(ctx context.Context, addr string) error {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(smiddleware.HTTPMiddleware(s.logger))

	r.Route(s.prefix, func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Get("/health", s.HealthCheckHandler)

			r.Group(func(r chi.Router) {
				r.Route("/task", func(r chi.Router) {
					r.Get("/", s.ToHTTPHandlerFunc(s.scheduler.GetOne))
					r.Post("/", s.ToHTTPHandlerFunc(s.scheduler.Insert))
					r.Delete("/", s.ToHTTPHandlerFunc(s.scheduler.Delete))
					r.Patch("/toggle", s.ToHTTPHandlerFunc(s.scheduler.Toggle))
				})

				r.Route("/helpers", func(r chi.Router) {
					r.Get("/active-tasks", s.ToHTTPHandlerFunc(s.scheduler.GetActive))
					r.Post("/execute-task", s.ToHTTPHandlerFunc(s.scheduler.Execute))
				})
			})
		})
	})

	errch := make(chan error)
	server := &http.Server{Addr: addr, Handler: r}
	go func() {
		s.logger.Info("Starting server", zap.String("addr", addr))
		errch <- server.ListenAndServe()
	}()

	select {
	case err := <-errch:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

// ToHTTPHandlerFunc converts a handler function to an http.HandlerFunc.
// This wrapper function is used to handle errors and respond to the client
func (s *Server) ToHTTPHandlerFunc(handler func(w http.ResponseWriter, r *http.Request) (any, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, status, err := handler(w, r)
		if err != nil {
			var errFromHandler *errors.Error
			switch {
			case errors.As(err, &errFromHandler):
				resp.RespondError(w, errFromHandler)
			default:
				s.logger.Error("internal error", zap.Error(errFromHandler))
				resp.RespondMessage(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		if response != nil {
			resp.RespondJSON(w, status, response)
		}
		if status >= 100 && status < 600 {
			w.WriteHeader(status)
		}
	}
}

// HealthCheckHandler returns the health status of the service
func (s *Server) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if ok := s.health.Health(r.Context()); !ok {
		resp.RespondMessage(w, http.StatusServiceUnavailable, "health check failed")
		return
	}
	resp.RespondMessage(w, http.StatusOK, "!!! We are RunninGoo !!!")
}
