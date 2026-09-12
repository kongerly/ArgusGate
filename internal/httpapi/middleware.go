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

// 使用包内私有 key 类型，避免与其他包放入 context 的字符串 key 冲突。
type contextKey string

const requestIDContextKey contextKey = "X-Request-ID"

// responseWriter 捕获首次提交的状态码，供请求结束日志记录实际响应状态。
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
		// 同一个 ID 同时进入响应头和 context，确保客户端与日志能够关联请求。
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

		// net/http 不暴露最终状态码，因此在不改变写入语义的前提下进行捕获。
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
		// Request ID 生成失败不应阻断探活等请求，固定占位值同时暴露异常状态。
		return "unknown"
	}

	return hex.EncodeToString(b[:])
}
