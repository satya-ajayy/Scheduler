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
	close     func()
	health    *health.HealthCheckService
	logger    *zap.Logger
	prefix    string
	scheduler *handlers.SchedulerHandler
}

func NewServer(
	logger *zap.Logger,
	prefix string,
	health *health.HealthCheckService,
	scheduler *handlers.SchedulerHandler,
	close func(),
) *Server {
	return &Server{
		close:     close,
		health:    health,
		logger:    logger,
		prefix:    prefix,
		scheduler: scheduler,
	}
}

func (s *Server) Listen(ctx context.Context, addr string) error {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(smiddleware.RequestLogger(s.logger))
	r.Use(middleware.Recoverer)

	r.Route(s.prefix, func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Get("/health", s.HealthCheckHandler)

			r.Group(func(r chi.Router) {
				r.Route("/task", func(r chi.Router) {
					r.Post("/", s.ToHTTPHandlerFunc(s.scheduler.Insert))
					r.Get("/{taskId}", s.ToHTTPHandlerFunc(s.scheduler.GetOne))
					r.Delete("/{taskId}", s.ToHTTPHandlerFunc(s.scheduler.Delete))
					r.Patch("/{taskId}/enable", s.ToHTTPHandlerFunc(s.scheduler.Enable))
					r.Patch("/{taskId}/disable", s.ToHTTPHandlerFunc(s.scheduler.Disable))
				})

				r.Route("/helpers", func(r chi.Router) {
					r.Get("/active-tasks", s.ToHTTPHandlerFunc(s.scheduler.GetActive))
					r.Post("/execute-task/{taskId}", s.ToHTTPHandlerFunc(s.scheduler.Execute))
				})
			})
		})
	})

	errch := make(chan error, 1)
	server := &http.Server{Addr: addr, Handler: r}
	go func() {
		s.logger.Info("Starting Server", zap.String("addr", addr))
		errch <- server.ListenAndServe()
	}()

	select {
	case err := <-errch:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if s.close != nil {
			s.close()
		}
		return nil
	}
}

// ToHTTPHandlerFunc converts a handler function to an http.HandlerFunc.
// This wrapper function is used to handle errors and respond to the client
func (s *Server) ToHTTPHandlerFunc(handler func(w http.ResponseWriter, r *http.Request) (any, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, status, err := handler(w, r)
		if err != nil {
			var typedErr *errors.Error
			switch {
			case errors.As(err, &typedErr):
				resp.RespondError(w, typedErr)
			default:
				s.logger.Error("Internal Error", zap.Error(err))
				resp.RespondMessage(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		if response != nil {
			resp.RespondJSON(w, status, response)
		} else if status >= 100 && status < 600 {
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
