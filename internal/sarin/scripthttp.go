package sarin

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/script"
	"go.aykhans.me/sarin/internal/types"
)

// scriptHTTPDefaultTimeout is the default timeout for script requests, independent of -T.
const scriptHTTPDefaultTimeout = 30 * time.Second

// scriptHTTPDefaultMaxBodySize is the default body limit for script requests,
// independent of the max response body config.
const scriptHTTPDefaultMaxBodySize = 10 << 20 // 10 MiB

// scriptHTTPClientKey picks the client a request needs, since fasthttp takes the body
// limit per client rather than per request.
type scriptHTTPClientKey struct {
	insecure    bool
	maxBodySize int
}

// scriptHTTPClient sends scripts' http.* requests. It is safe for concurrent use.
type scriptHTTPClient struct {
	ctx            context.Context //nolint:containedctx
	newClient      func(key scriptHTTPClientKey) *fasthttp.Client
	clients        sync.Map
	defaultTimeout time.Duration
}

var _ script.HTTPDoer = (*scriptHTTPClient)(nil)

// newScriptHTTPClients creates one client per proxy, in the same order as NewHostClients.
// It can return the following errors:
//   - types.ProxyDialError
func newScriptHTTPClients(ctx context.Context, proxies []url.URL, maxConns uint) ([]*scriptHTTPClient, error) {
	if len(proxies) == 0 {
		return []*scriptHTTPClient{newScriptHTTPClient(ctx, fasthttp.DialDualStackTimeout, scriptHTTPDefaultTimeout, maxConns)}, nil
	}

	clients := make([]*scriptHTTPClient, 0, len(proxies))
	for _, proxy := range proxies {
		dial, err := NewProxyDialFuncWithTimeout(ctx, &proxy)
		if err != nil {
			return nil, types.NewProxyDialError(proxy.String(), err)
		}
		clients = append(clients, newScriptHTTPClient(ctx, dial, scriptHTTPDefaultTimeout, maxConns))
	}
	return clients, nil
}

func newScriptHTTPClient(ctx context.Context, dial fasthttp.DialFuncWithTimeout, defaultTimeout time.Duration, maxConns uint) *scriptHTTPClient {
	return &scriptHTTPClient{
		ctx: ctx,
		newClient: func(key scriptHTTPClientKey) *fasthttp.Client {
			// Read and write timeouts stay unset because they would cap the per-call timeout,
			// so dialWithDeadline is what bounds the TLS handshake.
			return &fasthttp.Client{
				DialTimeout:         dialWithDeadline(dial),
				MaxConnsPerHost:     safeUintToInt(maxConns),
				MaxResponseBodySize: key.maxBodySize,
				TLSConfig: &tls.Config{
					InsecureSkipVerify: key.insecure, //nolint:gosec
				},
				DisableHeaderNamesNormalizing: true,
				DisablePathNormalizing:        true,
				NoDefaultUserAgentHeader:      true,
			}
		},
		defaultTimeout: defaultTimeout,
	}
}

// scriptHTTPBodyCap rounds a limit up to a power of two, so a script that varies its
// limit cannot mint a client, and a connection pool with it, per request.
func scriptHTTPBodyCap(maxBodySize int64) int {
	if maxBodySize <= 0 {
		return scriptHTTPDefaultMaxBodySize
	}

	size := int64(1)
	for size < maxBodySize {
		size <<= 1
	}
	return safeInt64ToInt(size)
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
	if err := c.ctx.Err(); err != nil {
		return nil, types.NewScriptHTTPRequestError(r.Method, r.URL, err)
	}

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

	client := c.clientFor(r.Insecure, r.MaxBodySize)

	req.SetTimeout(timeout)
	if r.MaxRedirects > 0 {
		err = client.DoRedirects(req, resp, r.MaxRedirects)
	} else {
		err = client.Do(req, resp)
	}
	if err != nil {
		if errors.Is(err, fasthttp.ErrBodyTooLarge) {
			err = types.ErrScriptHTTPBodyTooLarge
		}
		return nil, types.NewScriptHTTPRequestError(r.Method, r.URL, err)
	}

	// The client caps the body at the rounded up limit, so the exact one is checked here.
	body := resp.Body()
	if r.MaxBodySize > 0 && int64(len(body)) > r.MaxBodySize {
		return nil, types.NewScriptHTTPRequestError(r.Method, r.URL, types.ErrScriptHTTPBodyTooLarge)
	}

	return &script.HTTPResponse{
		Status:  resp.StatusCode(),
		Headers: collectRespHeaders(resp),
		Body:    string(body),
	}, nil
}

// clientFor returns the client whose body limit covers maxBodySize, building it once.
func (c *scriptHTTPClient) clientFor(insecure bool, maxBodySize int64) *fasthttp.Client {
	key := scriptHTTPClientKey{insecure: insecure, maxBodySize: scriptHTTPBodyCap(maxBodySize)}
	if client, ok := c.clients.Load(key); ok {
		return client.(*fasthttp.Client) //nolint:forcetypeassert
	}

	client, _ := c.clients.LoadOrStore(key, c.newClient(key))
	return client.(*fasthttp.Client) //nolint:forcetypeassert
}
