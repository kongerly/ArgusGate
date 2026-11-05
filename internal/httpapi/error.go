package httpapi

import (
	"encoding/json"
	"net/http"
)

type errorResponse struct {
	Error struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func writeJSONError(w http.ResponseWriter, r *http.Request, status int, errorType, code, message string) {
	if r.Context().Err() != nil {
		return
	}
	var response errorResponse
	response.Error.Message = message
	response.Error.Type = errorType
	response.Error.Code = code
	response.Error.RequestID = requestIDFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// 写入失败时响应已经提交，不能追加另一份错误。
	_ = json.NewEncoder(w).Encode(response)
}
