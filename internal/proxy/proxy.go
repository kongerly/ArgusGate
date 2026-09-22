package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type Proxy struct {
	client               *http.Client
	endpoint             *url.URL
	backendAuthorization string
}

// New 使用共享 HTTP Client、已校验的固定 endpoint 和可选的上游凭据创建代理。
// 调用方必须保证 client 与 endpoint 非空，并且不在 Proxy 生命周期内修改 endpoint。
func New(
	client *http.Client,
	endpoint *url.URL,
	backendAuthorization string,
) *Proxy {
	return &Proxy{
		client:               client,
		endpoint:             endpoint,
		backendAuthorization: backendAuthorization,
	}
}

// Forward 按原始字节构造上游 POST，并继承 ctx 的取消信号。
// 客户端的逐跳头和由网关托管的头不会透传；requestID 与上游凭据由此处显式设置。
// 成功返回后，response body 的关闭责任属于调用方。
func (p *Proxy) Forward(
	ctx context.Context,
	raw []byte,
	headers http.Header,
	requestID string,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.endpoint.String(),
		bytes.NewReader(raw),
	)
	if err != nil {
		return nil, fmt.Errorf("create upstream request: %w", err)
	}

	copyHeaders(req.Header, headers)

	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}

	if p.backendAuthorization != "" {
		req.Header.Set("Authorization", p.backendAuthorization)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send upstream request: %w", err)
	}

	return resp, nil
}
