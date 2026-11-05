package httpapi

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"

	"github.com/kongerly/ArgusGate/internal/proxy"
)

// Phase 1 固定使用默认上限；可配置限制留待对应阶段接入。
const maxRequestBodyBytes int64 = 4 << 20

func chatCompletions(logger *slog.Logger, p *proxy.Proxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSONError(w, r, http.StatusMethodNotAllowed, "invalid_request_error", "method_not_allowed", "only POST is allowed")
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeJSONError(w, r, http.StatusUnsupportedMediaType, "invalid_request_error", "unsupported_media_type", "Content-Type must be application/json")
			return
		}

		raw, err := readBody(r.Body, maxRequestBodyBytes)
		if err != nil {
			if errors.Is(err, errBodyTooLarge) {
				writeJSONError(w, r, http.StatusRequestEntityTooLarge, "invalid_request_error", "request_too_large", "request body exceeds 4 MiB limit")
			} else {
				writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error", "invalid_json", "could not read request body")
			}
			return
		}
		probe, err := parseRequestProbe(raw)
		if err != nil {
			writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error", "invalid_json", "request body contains invalid JSON")
			return
		}
		if err := validateProbe(probe); err != nil {
			if errors.Is(err, errInvalidModel) {
				writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error", "missing_model", "model must be a non-empty string")
			} else {
				writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error", "stream_not_supported", "stream is not supported in Phase 1")
			}
			return
		}

		requestID := requestIDFromContext(r.Context())
		resp, err := p.Forward(r.Context(), raw, r.Header, requestID)
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			// 不记录底层错误，避免其中的 URL 或凭据进入日志；精细分类在 D8 实现。
			logger.Error("upstream request failed", "request_id", requestID)
			writeJSONError(w, r, http.StatusBadGateway, "upstream_error", "upstream_connect_error", "upstream request failed")
			return
		}
		// WriteResponse 接管并关闭 body；此后错误只记录，不能再写网关错误。
		if err := proxy.WriteResponse(w, resp); err != nil {
			logger.Error("response forwarding failed", "request_id", requestID)
		}
	}
}
