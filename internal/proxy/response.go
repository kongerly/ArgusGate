package proxy

import (
	"fmt"
	"io"
	"net/http"
)

// WriteResponse 把 backend 的响应转发给客户端：
//   - 复制响应头（过滤 hop-by-hop 和 ArgusGate 管理的头）
//   - 原样写入 backend 的 status code
//   - 流式复制 body，不缓冲到内存
//
// 它接管 resp.Body 的所有权，无论成功失败都会关闭。
func WriteResponse(w http.ResponseWriter, resp *http.Response) error {
	defer resp.Body.Close()

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy response body: %w", err)
	}

	return nil
}
