package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var errBodyTooLarge = errors.New("body too large")

func readBody(r io.Reader, maxSize int64) ([]byte, error) {
	// 额外读取一个字节才能区分“恰好达到上限”和“已经超过上限”，同时避免无界读取。
	raw, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if int64(len(raw)) > maxSize {
		return nil, errBodyTooLarge
	}

	return raw, nil
}

type requestProbe struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

func parseRequestProbe(raw []byte) (requestProbe, error) {
	// 探针只提取网关决策所需字段；后续转发仍使用 raw，避免重编码改变未知字段或数值表示。
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return requestProbe{}, fmt.Errorf("request body is empty")
	}

	if trimmed[0] != '{' {
		return requestProbe{}, fmt.Errorf("request body must be a JSON object")
	}

	var probe requestProbe
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return requestProbe{}, fmt.Errorf("parse request probe: %w", err)
	}
	return probe, nil
}

var (
	errInvalidModel       = errors.New("model is required")
	errStreamNotSupported = errors.New("stream is not supported")
)

func validateProbe(p requestProbe) error {
	if strings.TrimSpace(p.Model) == "" {
		return errInvalidModel
	}

	if p.Stream {
		// Phase 1 只开放非流式路径；SSE 的 flush 与提交后错误语义将在 Phase 2 一并实现。
		return errStreamNotSupported
	}

	return nil
}
