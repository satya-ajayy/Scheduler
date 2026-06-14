package transport

import (
	"context"
	"net/http"
	"time"

	errors "scheduler/internal/errors"
	handler "scheduler/internal/transport/handler"
	reqlog "scheduler/internal/transport/middleware"
	response "scheduler/internal/transport/response"
	"scheduler/internal/version"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// HealthChecker is satisfied by any service that can report its health.
type HealthChecker interface {
	Health(ctx context.Context) error
}

type Server struct {
	close     func()
	health    HealthChecker
	logger    *zap.Logger
	prefix    string
	scheduler *handler.SchedulerHandler
}

func NewServer(
	logger *zap.Logger,
	prefix string,
	health HealthChecker,
	scheduler *handler.SchedulerHandler,
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
	r.Use(reqlog.RequestLogger(s.logger))
	r.Use(middleware.Recoverer)

	r.Route(s.prefix, func(r chi.Router) {
		r.Route("/v1", func(r chi.Router) {
			r.Get("/health", s.HealthCheckHandler)
			r.Get("/build", s.BuildInfoHandler)

			r.Group(func(r chi.Router) {
				r.Route("/task", func(r chi.Router) {
					r.Get("/{taskId}", s.ToHTTPHandlerFunc(s.scheduler.GetOne))
					r.Post("/", s.ToHTTPHandlerFunc(s.scheduler.Insert))
					r.Patch("/{taskId}/enable", s.ToHTTPHandlerFunc(s.scheduler.Enable))
					r.Patch("/{taskId}/disable", s.ToHTTPHandlerFunc(s.scheduler.Disable))
					r.Delete("/{taskId}", s.ToHTTPHandlerFunc(s.scheduler.Delete))
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
func (s *Server) ToHTTPHandlerFunc(handler func(w http.ResponseWriter, r *http.Request) (any, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, status, err := handler(w, r)
		if err != nil {
			var typedErr *errors.Error
			switch {
			case errors.As(err, &typedErr):
				response.RespondError(w, typedErr)
			default:
				s.logger.Error("Internal Error", zap.Error(err))
				response.RespondMessage(w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		if res != nil {
			response.RespondJSON(w, status, res)
		} else if status >= 100 && status < 600 {
			w.WriteHeader(status)
		}
	}
}

// BuildInfoHandler returns the build version and timestamp stamped at compile time.
func (s *Server) BuildInfoHandler(w http.ResponseWriter, r *http.Request) {
	response.RespondJSON(w, http.StatusOK, version.Get())
}

// HealthCheckHandler returns the health status of the service.
func (s *Server) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.health.Health(r.Context()); err != nil {
		s.logger.Error("Health Check Failed", zap.Error(err))
		response.RespondMessage(w, http.StatusServiceUnavailable, "health check failed")
		return
	}
	response.RespondMessage(w, http.StatusOK, "!!! We are RunninGoo !!!")
}
