package sarin

import (
	"compress/zlib"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"time"

	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/script"
	"go.aykhans.me/sarin/internal/types"
)

// scriptHTTPDefaultTimeout is the default timeout for script requests.
const scriptHTTPDefaultTimeout = 10 * time.Second

// scriptHTTPClient sends scripts' http.* requests. It is safe for concurrent use.
type scriptHTTPClient struct {
	ctx             context.Context //nolint:containedctx
	client          *fasthttp.Client
	insecureClient  *fasthttp.Client
	defaultTimeout  time.Duration
	maxResponseBody int
}

var _ script.HTTPDoer = (*scriptHTTPClient)(nil)

// newScriptHTTPClients creates one client per proxy, in the same order as NewHostClients.
// It can return the following errors:
//   - types.ProxyDialError
func newScriptHTTPClients(ctx context.Context, proxies []url.URL, maxConns uint, maxResponseBody uint64) ([]*scriptHTTPClient, error) {
	if len(proxies) == 0 {
		return []*scriptHTTPClient{newScriptHTTPClient(ctx, dialer.DialTimeout, scriptHTTPDefaultTimeout, maxConns, maxResponseBody)}, nil
	}

	clients := make([]*scriptHTTPClient, 0, len(proxies))
	for _, proxy := range proxies {
		dial, err := NewProxyDialFuncWithTimeout(ctx, &proxy)
		if err != nil {
			return nil, types.NewProxyDialError(proxy.String(), err)
		}
		clients = append(clients, newScriptHTTPClient(ctx, dial, scriptHTTPDefaultTimeout, maxConns, maxResponseBody))
	}
	return clients, nil
}

func newScriptHTTPClient(
	ctx context.Context,
	dial fasthttp.DialFuncWithTimeout,
	defaultTimeout time.Duration,
	maxConns uint,
	maxResponseBody uint64,
) *scriptHTTPClient {
	newClient := func(insecure bool) *fasthttp.Client {
		// Read and write timeouts stay unset because they would cap the per-call timeout,
		// so dialWithDeadline is what bounds the TLS handshake.
		return &fasthttp.Client{
			DialTimeout:               dialWithDeadline(dial),
			MaxConnsPerHost:           safeUintToInt(maxConns),
			MaxResponseBodySize:       safeUint64ToInt(maxResponseBody),
			MaxIdemponentCallAttempts: 1,
			ReadBufferSize:            64 << 10, // 64 KiB
			TLSConfig: &tls.Config{
				InsecureSkipVerify: insecure, //nolint:gosec
			},
			DisableHeaderNamesNormalizing: true,
			DisablePathNormalizing:        true,
			NoDefaultUserAgentHeader:      true,
		}
	}

	return &scriptHTTPClient{
		ctx:             ctx,
		client:          newClient(false),
		insecureClient:  newClient(true),
		defaultTimeout:  defaultTimeout,
		maxResponseBody: safeUint64ToInt(maxResponseBody),
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
		if errors.Is(err, fasthttp.ErrBodyTooLarge) {
			err = types.ErrResponseBodyTooLarge
		}
		return nil, types.NewScriptHTTPRequestError(r.Method, r.URL, err)
	}

	body, err := decodeResponseBody(resp, c.maxResponseBody)
	if err != nil {
		return nil, types.NewScriptHTTPRequestError(r.Method, r.URL, err)
	}

	return &script.HTTPResponse{
		Status:  resp.StatusCode(),
		Headers: collectRespHeaders(resp),
		Body:    string(body),
	}, nil
}

// decodeResponseBody decodes a body the server compressed, since scripts read it as text.
// The limit covers the decoded size, which is what a compression bomb inflates to.
// An empty body is left alone because the decoders reject zero bytes, and a HEAD or a 304
// carries the encoding with no body. A coding fasthttp does not decode, and the bare
// deflate stream some servers send without a zlib header, are left raw for the script.
func decodeResponseBody(resp *fasthttp.Response, maxResponseBody int) ([]byte, error) {
	if len(resp.Header.ContentEncoding()) == 0 || len(resp.Body()) == 0 {
		return resp.Body(), nil
	}

	decoded, err := resp.BodyUncompressedWithLimit(maxResponseBody)
	if errors.Is(err, fasthttp.ErrContentEncodingUnsupported) || errors.Is(err, zlib.ErrHeader) {
		return resp.Body(), nil
	}
	if err != nil {
		if errors.Is(err, fasthttp.ErrBodyTooLarge) {
			err = types.ErrResponseBodyTooLarge
		}
		return nil, err
	}

	// Neither header describes the decoded body, so both go, as net/http does on a gunzip.
	resp.Header.SetContentEncoding("")
	resp.Header.Del(fasthttp.HeaderContentLength)

	return decoded, nil
}
