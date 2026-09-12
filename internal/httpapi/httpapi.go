package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
)

func NewHandler(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthz)

	handler := loggingMiddleware(logger, mux)
	// Request ID 必须位于日志中间件外层，日志才能读取它写入的 request context。
	handler = requestIDMiddleware(handler)

	return handler
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	// 此时状态码已经提交；写入失败通常表示客户端断开，handler 无法再返回有效错误。
	_, _ = fmt.Fprintln(w, "OK")
}
