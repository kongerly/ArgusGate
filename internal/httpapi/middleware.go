package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

const requestIDHeader = "X-Request-ID"

type contextKey string

const requestIDContextKey contextKey = "X-Request-ID"

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(statusCode int) {
	// 与 net/http 一致，只记录并转发第一次提交的状态码。
	if w.wroteHeader {
		return
	}

	w.status = statusCode
	w.wroteHeader = true

	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	return w.ResponseWriter.Write(data)
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()

		w.Header().Set(requestIDHeader, requestID)

		ctx := context.WithValue(
			r.Context(),
			requestIDContextKey,
			requestID,
		)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey).(string)

	return requestID
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		next.ServeHTTP(rw, r)

		logger.Info(
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"request_id", requestIDFromContext(r.Context()),
			"duration", time.Since(start),
		)
	})
}

func newRequestID() string {
	var b [16]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return "unknown"
	}

	return hex.EncodeToString(b[:])
}
