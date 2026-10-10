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

// requestManagedHeaders 是 ArgusGate 在请求方向上自己管理的头。
// 客户端值不允许透传到 backend：
//   - Authorization：客户端凭据不能泄漏给 backend，由 Proxy 自己设置 backend credential
//   - X-Request-Id：客户端伪造的 ID 必须被丢弃，ArgusGate 用自己生成的 ID
var requestManagedHeaders = map[string]struct{}{
	"Authorization": {},
	"X-Request-Id":  {},
}

// responseManagedHeaders 是 ArgusGate 在响应方向上自己管理的头。
// backend 值不允许覆盖回客户端：
//   - X-Request-Id：客户端最终看到的必须是 ArgusGate 当前请求的 ID
//
// 这里不过滤 Authorization，因为响应方向不存在
// “客户端 credential 泄漏到 backend”的信任边界。
var responseManagedHeaders = map[string]struct{}{
	"X-Request-Id": {},
}

// buildSkipSet 返回一个完整的跳过集合，包含三部分：
//  1. 固定的 hop-by-hop 头
//  2. 当前方向的 managed 头
//  3. Connection 头里动态声明的头
//
// Connection 的查找使用 EqualFold 遍历 src，而不是 src.Values("Connection")，
// 因为 http.Header 可能由调用方直接构造，key 未必是规范形式。
func buildSkipSet(src http.Header, managed map[string]struct{}) map[string]struct{} {
	skip := make(map[string]struct{}, len(hopByHopHeaders)+len(managed))

	for k := range hopByHopHeaders {
		skip[k] = struct{}{}
	}

	for k := range managed {
		skip[k] = struct{}{}
	}

	for key, values := range src {
		if !strings.EqualFold(key, "Connection") {
			continue
		}
		for _, value := range values {
			for _, token := range strings.Split(value, ",") {
				name := strings.TrimSpace(token)
				if name == "" {
					continue
				}
				skip[http.CanonicalHeaderKey(name)] = struct{}{}
			}
		}
	}

	return skip
}

// copyFiltered 把 src 里允许的头复制到 dst。
//   - 跳过 buildSkipSet 返回的所有头
//   - 保留多值头，用 Add 而不是 Set
//   - key 统一做 canonical 化
func copyFiltered(dst, src http.Header, managed map[string]struct{}) {
	skip := buildSkipSet(src, managed)

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

// copyHeaders 复制请求方向的 end-to-end 头。
// 过滤 hop-by-hop、request-managed（Authorization / X-Request-ID）。
func copyHeaders(dst, src http.Header) {
	copyFiltered(dst, src, requestManagedHeaders)
}

// copyResponseHeaders 复制响应方向的 end-to-end 头。
// 过滤 hop-by-hop、response-managed（X-Request-ID）。
// Authorization 不过滤，因为响应方向不存在同样的信任边界。
func copyResponseHeaders(dst, src http.Header) {
	copyFiltered(dst, src, responseManagedHeaders)
}
