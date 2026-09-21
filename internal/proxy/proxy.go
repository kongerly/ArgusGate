package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Proxy 使用共享 HTTP Client 将请求转发到单个固定上游。
type Proxy struct {
	client   *http.Client
	endpoint *url.URL
}

// New 使用共享 HTTP Client 和已经过配置校验的固定 endpoint 创建代理。
// 调用方应保证二者非空，并在 Proxy 生命周期内不修改 endpoint。
func New(client *http.Client, endpoint *url.URL) *Proxy {
	return &Proxy{
		client:   client,
		endpoint: endpoint,
	}
}

// Forward 将原始请求体按原样发送到固定上游，并继承调用方 context 的取消信号。
// 成功返回后，response body 的关闭责任属于调用方。
func (p *Proxy) Forward(ctx context.Context, raw []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.endpoint.String(),
		bytes.NewReader(raw),
	)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send upstream request: %w", err)
	}

	return resp, nil
}
