package sarin

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"time"

	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/script"
	"go.aykhans.me/sarin/internal/types"
)

// scriptHTTPDefaultTimeout is the default timeout for script requests, independent of -T.
const scriptHTTPDefaultTimeout = 30 * time.Second

// scriptHTTPClient sends scripts' http.* requests. It is safe for concurrent use.
type scriptHTTPClient struct {
	client         *fasthttp.Client
	insecureClient *fasthttp.Client
	defaultTimeout time.Duration
}

var _ script.HTTPDoer = (*scriptHTTPClient)(nil)

// newScriptHTTPClients creates one client per proxy, in the same order as NewHostClients.
// It can return the following errors:
//   - types.ProxyDialError
func newScriptHTTPClients(ctx context.Context, proxies []url.URL, maxConns uint) ([]*scriptHTTPClient, error) {
	if len(proxies) == 0 {
		return []*scriptHTTPClient{newScriptHTTPClient(fasthttp.DialTimeout, scriptHTTPDefaultTimeout, maxConns)}, nil
	}

	clients := make([]*scriptHTTPClient, 0, len(proxies))
	for _, proxy := range proxies {
		dial, err := NewProxyDialFuncWithTimeout(ctx, &proxy)
		if err != nil {
			return nil, types.NewProxyDialError(proxy.String(), err)
		}
		clients = append(clients, newScriptHTTPClient(dial, scriptHTTPDefaultTimeout, maxConns))
	}
	return clients, nil
}

func newScriptHTTPClient(dial fasthttp.DialFuncWithTimeout, defaultTimeout time.Duration, maxConns uint) *scriptHTTPClient {
	newClient := func(insecure bool) *fasthttp.Client {
		// Read and write timeouts stay unset because they would cap the per-call timeout,
		// so dialWithDeadline is what bounds the TLS handshake.
		return &fasthttp.Client{
			DialTimeout:     dialWithDeadline(dial),
			MaxConnsPerHost: safeUintToInt(maxConns),
			TLSConfig: &tls.Config{
				InsecureSkipVerify: insecure, //nolint:gosec
			},
			DisableHeaderNamesNormalizing: true,
			DisablePathNormalizing:        true,
			NoDefaultUserAgentHeader:      true,
		}
	}

	return &scriptHTTPClient{
		client:         newClient(false),
		insecureClient: newClient(true),
		defaultTimeout: defaultTimeout,
	}
}

// dialWithDeadline dials within the request timeout and keeps it as the connection deadline,
// so the TLS handshake fasthttp runs on the first write cannot outlive the request.
func dialWithDeadline(dial fasthttp.DialFuncWithTimeout) fasthttp.DialFuncWithTimeout {
	return func(addr string, timeout time.Duration) (net.Conn, error) {
		if timeout <= 0 {
			timeout = scriptHTTPDefaultTimeout
		}
		deadline := time.Now().Add(timeout)

		conn, err := dial(addr, timeout)
		if err != nil {
			return nil, err
		}
		if err := conn.SetDeadline(deadline); err != nil {
			conn.Close() //nolint:errcheck,gosec
			return nil, err
		}
		return conn, nil
	}
}

// Do sends the request. With maxRedirects the timeout applies to each hop.
// It can return the following errors:
//   - types.ScriptHTTPRequestError
func (c *scriptHTTPClient) Do(r *script.HTTPRequest) (*script.HTTPResponse, error) {
	parsedURL, err := url.Parse(r.URL)
	if err != nil {
		return nil, types.NewScriptHTTPRequestError(r.Method, r.URL, err)
	}
	if (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, types.NewScriptHTTPRequestError(r.Method, r.URL, types.ErrScriptHTTPURLInvalid)
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(r.URL)
	req.Header.SetMethod(r.Method)
	for key, values := range r.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if len(r.Params) > 0 {
		args := req.URI().QueryArgs()
		for key, values := range r.Params {
			for _, value := range values {
				args.Add(key, value)
			}
		}
	}

	if len(r.Cookies) > 0 {
		req.Header.Add("Cookie", cookieHeaderValue(r.Cookies))
	}

	req.SetBodyString(r.Body)

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = c.defaultTimeout
	}

	client := c.client
	if r.Insecure {
		client = c.insecureClient
	}

	req.SetTimeout(timeout)
	if r.MaxRedirects > 0 {
		err = client.DoRedirects(req, resp, r.MaxRedirects)
	} else {
		err = client.Do(req, resp)
	}
	if err != nil {
		return nil, types.NewScriptHTTPRequestError(r.Method, r.URL, err)
	}

	return &script.HTTPResponse{
		Status:  resp.StatusCode(),
		Headers: collectRespHeaders(resp),
		Body:    string(resp.Body()),
	}, nil
}
