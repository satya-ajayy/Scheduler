package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// RequestLogger creates a middleware that logs HTTP requests using zap
func RequestLogger(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Start tracking request duration and set up response logging
			start := time.Now()
			reqID := middleware.GetReqID(r.Context())
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Create a child logger with request-specific fields
			requestLogger := logger.With(
				zap.String("reqId", reqID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
			)

			// Process the request
			next.ServeHTTP(ww, r)

			// Log the request details after it's processed
			duration := time.Since(start)
			if ww.Status() == http.StatusOK && isDebugLog(r) {
				requestLogger.Debug("Served", zap.Int("status", ww.Status()),
					zap.Duration("duration", duration), zap.Int("size", ww.BytesWritten()))
			} else {
				requestLogger.Info("Served", zap.Int("status", ww.Status()),
					zap.Duration("duration", duration), zap.Int("size", ww.BytesWritten()))
			}
		})
	}
}

func isDebugLog(r *http.Request) bool {
	paths := []string{"/health", "/metrics"}
	for _, path := range paths {
		if strings.HasSuffix(r.URL.Path, path) {
			return true
		}
	}
	return false
}
