package types

import (
	"net/url"
)

type Proxy url.URL

func (proxy Proxy) String() string {
	return (*url.URL)(&proxy).String()
}

type Proxies []Proxy

func (proxies *Proxies) Append(proxy ...Proxy) {
	*proxies = append(*proxies, proxy...)
}

// Parse parses a raw proxy string and appends it to the list.
// It can return the following errors:
//   - ProxyParseError
func (proxies *Proxies) Parse(rawValue string) error {
	parsedProxy, err := ParseProxy(rawValue)
	if err != nil {
		return err
	}

	proxies.Append(*parsedProxy)
	return nil
}

// ParseProxy parses a raw proxy URL string into a Proxy.
// It can return the following errors:
//   - ProxyParseError
func ParseProxy(rawValue string) (*Proxy, error) {
	urlParsed, err := url.Parse(rawValue)
	if err != nil {
		return nil, NewProxyParseError(err)
	}

	proxyParsed := Proxy(*urlParsed)
	return &proxyParsed, nil
}
