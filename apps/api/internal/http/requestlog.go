package http

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/clientip"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/logging"
)

// RequestIDHeader carries the request ID; a well-formed incoming value (e.g. from a load
// balancer) is kept so logs can be correlated across hops.
const RequestIDHeader = "X-Request-ID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,64}$`)

func newRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// requestLogging assigns a request ID, recovers panics and writes one access log line per
// request. Only the method, path, route, status, size, duration and client address are logged:
// never headers (Authorization, cookies), query strings or bodies, which can hold credentials.
func requestLogging(logger *slog.Logger, resolver *clientip.Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			id := r.Header.Get(RequestIDHeader)
			if !validRequestID.MatchString(id) {
				id = newRequestID()
			}
			w.Header().Set(RequestIDHeader, id)
			ctx := logging.WithRequest(r.Context(), id)
			r = r.WithContext(ctx)
			rec := &statusRecorder{ResponseWriter: w}

			defer func() {
				if p := recover(); p != nil {
					if p == http.ErrAbortHandler {
						panic(p)
					}
					logger.ErrorContext(ctx, "panic while handling request", slog.Any("panic", p))
					if !rec.wroteHeader {
						writeError(rec, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
					}
				}

				status := rec.status
				if status == 0 {
					status = http.StatusOK
				}
				level := slog.LevelInfo
				if status >= 500 {
					level = slog.LevelError
				}
				route := ""
				if rc := chi.RouteContext(ctx); rc != nil {
					route = rc.RoutePattern()
				}
				logger.LogAttrs(ctx, level, "request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("route", route),
					slog.Int("status", status),
					slog.Int64("bytes", rec.bytes),
					slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
					slog.String("client_ip", resolver.ClientIP(r)),
				)
			}()

			next.ServeHTTP(rec, r)
		})
	}
}

// statusRecorder remembers the status code and body size written by handlers.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
