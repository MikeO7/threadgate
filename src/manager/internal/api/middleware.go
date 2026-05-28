package api

import (
	"log"
	"net/http"
	"time"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs details of each incoming HTTP request.
func (s *Server) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := newLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)
		duration := time.Since(start)
		//nolint:gosec // G706: Request logging is standard and safe
		log.Printf("[API Server] %s %s from %s -> %d %s (%v)\n",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			lrw.statusCode,
			http.StatusText(lrw.statusCode),
			duration,
		)
	})
}
