package script

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.aykhans.me/sarin/internal/types"
)

// HTTPRequest is a request made by a script through the http.* functions.
type HTTPRequest struct {
	Method       string
	URL          string
	Headers      map[string][]string
	Params       map[string][]string
	Cookies      map[string][]string
	Body         string
	Timeout      time.Duration // zero means the doer's default timeout
	Insecure     bool
	MaxRedirects int
	MaxBodySize  int64 // zero means no limit
}

// HTTPResponse is the response returned to a script by the http.* functions.
type HTTPResponse struct {
	Status  int
	Headers map[string][]string
	Body    string
}

// Header returns the first value of the named header, matching the name case-insensitively.
func (r *HTTPResponse) Header(name string) (string, bool) {
	for key, values := range r.Headers {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0], true
		}
	}
	return "", false
}

// Cookie returns the value of the named cookie from the Set-Cookie headers.
func (r *HTTPResponse) Cookie(name string) (string, bool) {
	for key, values := range r.Headers {
		if !strings.EqualFold(key, "Set-Cookie") {
			continue
		}
		for _, line := range values {
			if cookie, err := http.ParseSetCookie(line); err == nil && cookie.Name == name {
				return cookie.Value, true
			}
		}
	}
	return "", false
}

// HTTPDoer sends the HTTP requests that scripts make.
type HTTPDoer interface {
	Do(req *HTTPRequest) (*HTTPResponse, error)
}

// httpDefaultMethod is used when the options set no method.
const httpDefaultMethod = http.MethodGet

// httpMaxRedirectsLimit is the highest maxRedirects a script may ask for.
const httpMaxRedirectsLimit = 100

// httpMaxBodySizeLimit is the highest maxBodySize a script may ask for.
const httpMaxBodySizeLimit int64 = types.ByteSizeLimit

// Option keys accepted by the http function.
const (
	httpOptionMethod       = "method"
	httpOptionHeaders      = "headers"
	httpOptionParams       = "params"
	httpOptionCookies      = "cookies"
	httpOptionBody         = "body"
	httpOptionTimeout      = "timeout"
	httpOptionInsecure     = "insecure"
	httpOptionMaxRedirects = "maxRedirects"
	httpOptionMaxBodySize  = "maxBodySize"
)

// httpBridge connects http to the doer. Calls only work while transform runs.
type httpBridge struct {
	doer   HTTPDoer
	active bool
}

// It can return the following errors:
//   - types.ErrScriptHTTPOutsideTransform
//   - types.ErrScriptHTTPUnavailable
//   - types.ErrScriptHTTPMethodEmpty
//   - any error returned by the doer
func (b *httpBridge) do(req *HTTPRequest) (*HTTPResponse, error) {
	if !b.active {
		return nil, types.ErrScriptHTTPOutsideTransform
	}
	if b.doer == nil {
		return nil, types.ErrScriptHTTPUnavailable
	}
	if req.Method == "" {
		return nil, types.ErrScriptHTTPMethodEmpty
	}
	return b.doer.Do(req)
}

// parseHTTPMaxRedirects checks that the redirect limit is a non-negative whole number.
// It can return the following errors:
//   - types.ScriptHTTPOptionError
func parseHTTPMaxRedirects(count float64) (int, error) {
	if math.IsNaN(count) || count < 0 || count > httpMaxRedirectsLimit || count != math.Trunc(count) {
		return 0, types.NewScriptHTTPOptionError(
			httpOptionMaxRedirects,
			types.NewScriptTypeError(
				"a whole number between 0 and "+strconv.Itoa(httpMaxRedirectsLimit),
				strconv.FormatFloat(count, 'g', -1, 64),
			),
		)
	}
	return int(count), nil
}

// parseHTTPMaxBodySize checks that the body limit is a non-negative whole number.
// It can return the following errors:
//   - types.ScriptHTTPOptionError
func parseHTTPMaxBodySize(size float64) (int64, error) {
	if math.IsNaN(size) || size < 0 || size > float64(httpMaxBodySizeLimit) || size != math.Trunc(size) {
		return 0, types.NewScriptHTTPOptionError(
			httpOptionMaxBodySize,
			types.NewScriptTypeError(
				"a whole number of bytes between 0 and "+strconv.FormatInt(httpMaxBodySizeLimit, 10),
				strconv.FormatFloat(size, 'f', -1, 64),
			),
		)
	}
	return int64(size), nil
}

// parseHTTPMaxBodySizeString reads a body limit written with a unit, such as "10MiB".
// It can return the following errors:
//   - types.ScriptHTTPOptionError
func parseHTTPMaxBodySizeString(value string) (int64, error) {
	size, err := types.ParseByteSize(value)
	if err != nil {
		return 0, types.NewScriptHTTPOptionError(httpOptionMaxBodySize, err)
	}
	if size > uint64(httpMaxBodySizeLimit) {
		return 0, types.NewScriptHTTPOptionError(
			httpOptionMaxBodySize,
			types.NewScriptTypeError("a size up to "+types.FormatByteSize(uint64(httpMaxBodySizeLimit)), value),
		)
	}
	return int64(size), nil
}

// parseHTTPTimeout parses a Go duration string such as "500ms" or "2s".
// It can return the following errors:
//   - types.ScriptHTTPOptionError
func parseHTTPTimeout(value string) (time.Duration, error) {
	timeout, err := time.ParseDuration(value)
	if err != nil {
		return 0, types.NewScriptHTTPOptionError(httpOptionTimeout, err)
	}
	if timeout <= 0 {
		return 0, types.NewScriptHTTPOptionError(httpOptionTimeout, types.NewScriptTypeError("a duration greater than 0", value))
	}
	return timeout, nil
}
