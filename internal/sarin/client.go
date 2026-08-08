package sarin

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"math"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
	"go.aykhans.me/sarin/internal/types"
	utilsSlice "go.aykhans.me/utils/slice"
	"golang.org/x/net/proxy"
)

type HostClientGenerator func() *fasthttp.HostClient

// HostClientsFactory builds the host clients a single worker will use. Each
// worker gets its own set so the per-client connection pool is never shared:
// fasthttp guards that pool with a mutex and, with the default FIFO strategy,
// shifts the whole pool on every acquire, which turns into per-request
// contention proportional to the worker count once the clients are shared.
type HostClientsFactory func() []*fasthttp.HostClient

func safeUintToInt(u uint) int {
	if u > math.MaxInt {
		return math.MaxInt
	}
	return int(u)
}

// newHostClient builds a single client. maxConns bounds the client's own pool;
// MaxConnDuration is deliberately left at zero so keep-alive connections live
// for the whole run. Bounding it makes fasthttp force a `Connection: close` and
// redial once the bound is passed, which costs a TCP (and, over HTTPS, a TLS)
// handshake on that request and shows up as a latency spike in the stats.
func newHostClient(
	addr string,
	isTLS bool,
	tlsConfig *tls.Config,
	dialFunc fasthttp.DialFunc,
	maxConns int,
	timeout time.Duration,
) *fasthttp.HostClient {
	return &fasthttp.HostClient{
		MaxConns:                      maxConns,
		IsTLS:                         isTLS,
		TLSConfig:                     tlsConfig,
		Addr:                          addr,
		Dial:                          dialFunc,
		ConnPoolStrategy:              fasthttp.LIFO,
		MaxIdleConnDuration:           timeout,
		WriteTimeout:                  timeout,
		ReadTimeout:                   timeout,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		NoDefaultUserAgentHeader:      true,
	}
}

// NewHostClients returns a factory that creates the fasthttp.HostClient set for
// one worker. If no proxies are provided the worker gets a single client.
//
// Proxy dial functions are built (and validated) once here and shared by every
// worker; they are stateless and safe for concurrent use.
//
// It can return the following errors:
// - types.ProxyDialError
func NewHostClients(
	ctx context.Context,
	timeout time.Duration,
	proxies []url.URL,
	maxConns uint,
	requestURL *url.URL,
	skipVerify bool,
) (HostClientsFactory, error) {
	isTLS := requestURL.Scheme == "https"
	// Shared across clients: fasthttp clones it per address before use, so it is
	// only ever read.
	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipVerify, //nolint:gosec
	}

	if proxiesLen := len(proxies); proxiesLen > 0 {
		addr := requestURL.Host
		if isTLS && requestURL.Port() == "" {
			addr += ":443"
		}

		dialFuncs := make([]fasthttp.DialFunc, 0, proxiesLen)
		for _, proxy := range proxies {
			dialFunc, err := NewProxyDialFunc(ctx, &proxy, timeout)
			if err != nil {
				return nil, types.NewProxyDialError(proxy.String(), err)
			}
			dialFuncs = append(dialFuncs, dialFunc)
		}

		// With proxies a worker picks a different client per request, so a
		// per-worker client set would open one connection per worker per proxy.
		// The clients stay shared here and only the pool strategy is tuned.
		clients := make([]*fasthttp.HostClient, 0, proxiesLen)
		for _, dialFunc := range dialFuncs {
			clients = append(clients, newHostClient(addr, isTLS, tlsConfig, dialFunc, safeUintToInt(maxConns), timeout))
		}

		return func() []*fasthttp.HostClient { return clients }, nil
	}

	// Without proxies every worker drives exactly one connection, so each gets
	// its own single-connection client and never touches a shared pool.
	return func() []*fasthttp.HostClient {
		return []*fasthttp.HostClient{
			newHostClient(requestURL.Host, isTLS, tlsConfig, nil, 1, timeout),
		}
	}, nil
}

// NewProxyDialFunc creates a dial function for the given proxy URL.
// It can return the following errors:
//   - types.ProxyUnsupportedSchemeError
func NewProxyDialFunc(ctx context.Context, proxyURL *url.URL, timeout time.Duration) (fasthttp.DialFunc, error) {
	var (
		dialer fasthttp.DialFunc
		err    error
	)

	switch proxyURL.Scheme {
	case "socks5":
		dialer, err = fasthttpSocksDialerDualStackTimeout(ctx, proxyURL, timeout, true)
		if err != nil {
			return nil, err
		}
	case "socks5h":
		dialer, err = fasthttpSocksDialerDualStackTimeout(ctx, proxyURL, timeout, false)
		if err != nil {
			return nil, err
		}
	case "http":
		dialer = fasthttpproxy.FasthttpHTTPDialerDualStackTimeout(proxyURL.String(), timeout)
	case "https":
		dialer = fasthttpHTTPSDialerDualStackTimeout(proxyURL, timeout)
	default:
		return nil, types.NewProxyUnsupportedSchemeError(proxyURL.Scheme)
	}

	return dialer, nil
}

// The returned dial function can return the following errors:
//   - types.ProxyDialError
func fasthttpSocksDialerDualStackTimeout(ctx context.Context, proxyURL *url.URL, timeout time.Duration, resolveLocally bool) (fasthttp.DialFunc, error) {
	netDialer := &net.Dialer{}

	// Parse auth from proxy URL if present
	var auth *proxy.Auth
	if proxyURL.User != nil {
		auth = &proxy.Auth{
			User: proxyURL.User.Username(),
		}
		if password, ok := proxyURL.User.Password(); ok {
			auth.Password = password
		}
	}

	// Create SOCKS5 dialer with net.Dialer as forward dialer
	socksDialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, netDialer)
	if err != nil {
		return nil, err
	}

	proxyStr := proxyURL.String()

	// Assert to ContextDialer for timeout support
	contextDialer, ok := socksDialer.(proxy.ContextDialer)
	if !ok {
		// Fallback without timeout (should not happen with net.Dialer)
		return func(addr string) (net.Conn, error) {
			conn, err := socksDialer.Dial("tcp", addr)
			if err != nil {
				return nil, types.NewProxyDialError(proxyStr, err)
			}
			return conn, nil
		}, nil
	}

	// Return dial function that uses context with timeout
	return func(addr string) (net.Conn, error) {
		deadline := time.Now().Add(timeout)

		if resolveLocally {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, types.NewProxyDialError(proxyStr, err)
			}

			dnsCtx, dnsCancel := context.WithTimeout(ctx, timeout)
			ips, err := net.DefaultResolver.LookupIP(dnsCtx, "ip", host)
			dnsCancel()
			if err != nil {
				return nil, types.NewProxyDialError(proxyStr, err)
			}
			if len(ips) == 0 {
				return nil, types.NewProxyDialError(proxyStr, types.NewProxyResolveError(host))
			}

			// Use the first resolved IP
			addr = net.JoinHostPort(ips[0].String(), port)
		}

		// Use remaining time for dial
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, types.NewProxyDialError(proxyStr, context.DeadlineExceeded)
		}

		dialCtx, dialCancel := context.WithTimeout(ctx, remaining)
		defer dialCancel()

		conn, err := contextDialer.DialContext(dialCtx, "tcp", addr)
		if err != nil {
			return nil, types.NewProxyDialError(proxyStr, err)
		}
		return conn, nil
	}, nil
}

// The returned dial function can return the following errors:
//   - types.ProxyDialError
func fasthttpHTTPSDialerDualStackTimeout(proxyURL *url.URL, timeout time.Duration) fasthttp.DialFunc {
	proxyAddr := proxyURL.Host
	if proxyURL.Port() == "" {
		proxyAddr = net.JoinHostPort(proxyURL.Hostname(), "443")
	}

	// Build Proxy-Authorization header if auth is present
	var proxyAuth string
	if proxyURL.User != nil {
		username := proxyURL.User.Username()
		password, _ := proxyURL.User.Password()
		credentials := username + ":" + password
		proxyAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
	}

	proxyStr := proxyURL.String()

	return func(addr string) (net.Conn, error) {
		// Establish TCP connection to proxy with timeout
		start := time.Now()
		conn, err := fasthttp.DialDualStackTimeout(proxyAddr, timeout)
		if err != nil {
			return nil, types.NewProxyDialError(proxyStr, err)
		}

		remaining := timeout - time.Since(start)
		if remaining <= 0 {
			conn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, context.DeadlineExceeded)
		}

		// Set deadline for the TLS handshake and CONNECT request
		if err := conn.SetDeadline(time.Now().Add(remaining)); err != nil {
			conn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, err)
		}

		// Upgrade to TLS
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName: proxyURL.Hostname(),
		})
		if err := tlsConn.Handshake(); err != nil {
			tlsConn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, err)
		}

		// Build and send CONNECT request
		connectReq := &http.Request{
			Method: http.MethodConnect,
			URL:    &url.URL{Opaque: addr},
			Host:   addr,
			Header: make(http.Header),
		}
		if proxyAuth != "" {
			connectReq.Header.Set("Proxy-Authorization", proxyAuth)
		}

		if err := connectReq.Write(tlsConn); err != nil {
			tlsConn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, err)
		}

		// Read response using buffered reader, but return wrapped connection
		// to preserve any buffered data
		bufReader := bufio.NewReader(tlsConn)
		resp, err := http.ReadResponse(bufReader, connectReq)
		if err != nil {
			tlsConn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, err)
		}
		resp.Body.Close() //nolint:errcheck,gosec

		if resp.StatusCode != http.StatusOK {
			tlsConn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, types.NewProxyConnectError(resp.Status))
		}

		// Clear deadline for the tunneled connection
		if err := tlsConn.SetDeadline(time.Time{}); err != nil {
			tlsConn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, err)
		}

		// Return wrapped connection that uses the buffered reader
		// to avoid losing any data that was read ahead
		return &bufferedConn{Conn: tlsConn, reader: bufReader}, nil
	}
}

// bufferedConn wraps a net.Conn with a buffered reader to preserve
// any data that was read during HTTP response parsing.
type bufferedConn struct {
	net.Conn

	reader *bufio.Reader
}

func (c *bufferedConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

func NewHostClientGenerator(clients ...*fasthttp.HostClient) HostClientGenerator {
	switch len(clients) {
	case 0:
		hostClient := &fasthttp.HostClient{}
		return func() *fasthttp.HostClient {
			return hostClient
		}
	case 1:
		return func() *fasthttp.HostClient {
			return clients[0]
		}
	default:
		return utilsSlice.RandomCycle(nil, clients...)
	}
}
