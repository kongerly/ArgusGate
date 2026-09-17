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
		return errStreamNotSupported
	}

	return nil
}
