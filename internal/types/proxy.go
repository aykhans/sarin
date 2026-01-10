package types

import (
	"fmt"
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

func (proxies *Proxies) Parse(rawValue string) error {
	parsedProxy, err := ParseProxy(rawValue)
	if err != nil {
		return err
	}

	proxies.Append(*parsedProxy)
	return nil
}

func ParseProxy(rawValue string) (*Proxy, error) {
	urlParsed, err := url.Parse(rawValue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
	}

	proxyParsed := Proxy(*urlParsed)
	return &proxyParsed, nil
}
