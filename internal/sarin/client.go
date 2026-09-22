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
	"go.aykhans.me/sarin/internal/types"
	utilsSlice "go.aykhans.me/utils/slice"
	"golang.org/x/net/proxy"
)

// clientIndexGenerator returns the index of the host client (and proxy) to use next.
type clientIndexGenerator func() int

func safeUintToInt(u uint) int {
	if u > math.MaxInt {
		return math.MaxInt
	}
	return int(u)
}

// NewHostClients creates a list of fasthttp.HostClient instances for the given proxies.
// If no proxies are provided, a single client without a proxy is returned.
// It can return the following errors:
// - types.ProxyDialError
func NewHostClients(
	ctx context.Context,
	timeout time.Duration,
	proxies []url.URL,
	maxConns uint,
	requestURL *url.URL,
	skipVerify bool,
) ([]*fasthttp.HostClient, error) {
	isTLS := requestURL.Scheme == "https"

	if proxiesLen := len(proxies); proxiesLen > 0 {
		clients := make([]*fasthttp.HostClient, 0, proxiesLen)
		addr := requestURL.Host
		if isTLS && requestURL.Port() == "" {
			addr += ":443"
		}

		for _, proxy := range proxies {
			dialFunc, err := NewProxyDialFunc(ctx, &proxy, timeout)
			if err != nil {
				return nil, types.NewProxyDialError(proxy.String(), err)
			}

			clients = append(clients, &fasthttp.HostClient{
				MaxConns: safeUintToInt(maxConns),
				IsTLS:    isTLS,
				TLSConfig: &tls.Config{
					InsecureSkipVerify: skipVerify, //nolint:gosec
				},
				Addr:                          addr,
				Dial:                          dialFunc,
				MaxIdleConnDuration:           timeout,
				MaxConnDuration:               timeout,
				WriteTimeout:                  timeout,
				ReadTimeout:                   timeout,
				DisableHeaderNamesNormalizing: true,
				DisablePathNormalizing:        true,
				NoDefaultUserAgentHeader:      true,
			},
			)
		}

		return clients, nil
	}

	client := &fasthttp.HostClient{
		MaxConns: safeUintToInt(maxConns),
		IsTLS:    isTLS,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: skipVerify, //nolint:gosec
		},
		Addr:                          requestURL.Host,
		MaxIdleConnDuration:           timeout,
		MaxConnDuration:               timeout,
		WriteTimeout:                  timeout,
		ReadTimeout:                   timeout,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		NoDefaultUserAgentHeader:      true,
	}
	return []*fasthttp.HostClient{client}, nil
}

// NewProxyDialFunc creates a dial function for the given proxy URL with a fixed timeout.
// It can return the following errors:
//   - types.ProxyUnsupportedSchemeError
//   - types.ErrProxyNoContextDialer
func NewProxyDialFunc(ctx context.Context, proxyURL *url.URL, timeout time.Duration) (fasthttp.DialFunc, error) {
	dial, err := NewProxyDialFuncWithTimeout(ctx, proxyURL)
	if err != nil {
		return nil, err
	}

	return func(addr string) (net.Conn, error) {
		return dial(addr, timeout)
	}, nil
}

// NewProxyDialFuncWithTimeout creates a dial function for the given proxy URL that takes the timeout per call.
// It can return the following errors:
//   - types.ProxyUnsupportedSchemeError
//   - types.ErrProxyNoContextDialer
func NewProxyDialFuncWithTimeout(ctx context.Context, proxyURL *url.URL) (fasthttp.DialFuncWithTimeout, error) {
	switch proxyURL.Scheme {
	case "socks5":
		return fasthttpSocksDialer(ctx, proxyURL, true)
	case "socks5h":
		return fasthttpSocksDialer(ctx, proxyURL, false)
	case "http":
		return fasthttpConnectDialer(proxyURL, false), nil
	case "https":
		return fasthttpConnectDialer(proxyURL, true), nil
	default:
		return nil, types.NewProxyUnsupportedSchemeError(proxyURL.Scheme)
	}
}

// fasthttpSocksDialer creates a SOCKS5 dial function that takes the timeout per call.
// It can return the following errors:
//   - types.ErrProxyNoContextDialer
//   - types.ProxyDialError
func fasthttpSocksDialer(ctx context.Context, proxyURL *url.URL, resolveLocally bool) (fasthttp.DialFuncWithTimeout, error) {
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

	// Timeouts need DialContext, which proxy.SOCKS5 always provides with a net.Dialer.
	contextDialer, ok := socksDialer.(proxy.ContextDialer)
	if !ok {
		return nil, types.ErrProxyNoContextDialer
	}

	// Return dial function that uses context with timeout
	return func(addr string, timeout time.Duration) (net.Conn, error) {
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

// fasthttpConnectDialer creates a dial function that tunnels through an HTTP proxy with CONNECT,
// reaching the proxy over TLS when useTLS is set.
//
// The returned dial function can return the following errors:
//   - types.ProxyDialError
func fasthttpConnectDialer(proxyURL *url.URL, useTLS bool) fasthttp.DialFuncWithTimeout {
	defaultPort := "80"
	if useTLS {
		defaultPort = "443"
	}

	proxyAddr := proxyURL.Host
	if proxyURL.Port() == "" {
		proxyAddr = net.JoinHostPort(proxyURL.Hostname(), defaultPort)
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

	return func(addr string, timeout time.Duration) (net.Conn, error) {
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

		if useTLS {
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName: proxyURL.Hostname(),
			})
			if err := tlsConn.Handshake(); err != nil {
				tlsConn.Close() //nolint:errcheck,gosec
				return nil, types.NewProxyDialError(proxyStr, err)
			}
			conn = tlsConn
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

		if err := connectReq.Write(conn); err != nil {
			conn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, err)
		}

		// Read response using buffered reader, but return wrapped connection
		// to preserve any buffered data
		bufReader := bufio.NewReader(conn)
		resp, err := http.ReadResponse(bufReader, connectReq)
		if err != nil {
			conn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, err)
		}
		resp.Body.Close() //nolint:errcheck,gosec

		if resp.StatusCode != http.StatusOK {
			conn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, types.NewProxyConnectError(resp.Status))
		}

		// Clear deadline for the tunneled connection
		if err := conn.SetDeadline(time.Time{}); err != nil {
			conn.Close() //nolint:errcheck,gosec
			return nil, types.NewProxyDialError(proxyStr, err)
		}

		// Return wrapped connection that uses the buffered reader
		// to avoid losing any data that was read ahead
		return &bufferedConn{Conn: conn, reader: bufReader}, nil
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

func newClientIndexGenerator(count int) clientIndexGenerator {
	indexes := make([]int, count)
	for i := range indexes {
		indexes[i] = i
	}
	return utilsSlice.RandomCycle(nil, indexes...)
}
