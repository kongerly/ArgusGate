package proxy

import (
	"net/http"
	"strings"
)

// hopByHopHeaders 是每一跳都必须消费掉、不能转发的 HTTP 头。
// 除了 RFC 7230 定义的标准集合，还包含现实代理环境中常见的
// Proxy-Connection（非标准，但 Go 标准库的反向代理也会过滤它）。
var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Proxy-Connection":    {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

// managedHeaders 是 ArgusGate 自己管理、不允许从客户端透传的头。
// 这些头由 Forward 显式设置，copyHeaders 必须直接跳过，
// 否则会出现“先复制、后覆盖”的脏路径。
var managedHeaders = map[string]struct{}{
	"Authorization": {},
	"X-Request-Id":  {},
}

// buildSkipSet 返回一个完整的跳过集合，包含三部分：
//  1. 固定的 hop-by-hop 头
//  2. Connection 头里动态声明的头
//  3. ArgusGate 自己管理的头
func buildSkipSet(src http.Header) map[string]struct{} {
	skip := make(map[string]struct{}, len(hopByHopHeaders)+len(managedHeaders))

	for k := range hopByHopHeaders {
		skip[k] = struct{}{}
	}

	for k := range managedHeaders {
		skip[k] = struct{}{}
	}

	for _, value := range src.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			name := strings.TrimSpace(token)
			if name == "" {
				continue
			}
			skip[http.CanonicalHeaderKey(name)] = struct{}{}
		}
	}

	return skip
}

// copyHeaders 把 src 里允许的头复制到 dst。
//   - 跳过 buildSkipSet 返回的所有头
//   - 保留多值头，用 Add 而不是 Set
//   - key 统一做 canonical 化
func copyHeaders(dst, src http.Header) {
	skip := buildSkipSet(src)

	for key, values := range src {
		canonicalKey := http.CanonicalHeaderKey(key)
		if _, blocked := skip[canonicalKey]; blocked {
			continue
		}

		for _, value := range values {
			dst.Add(canonicalKey, value)
		}
	}
}
