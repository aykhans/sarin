package types

import "strings"

type Cookie KeyValue[string, []string]

type Cookies []Cookie

func (cookies Cookies) GetValue(key string) *[]string {
	for i := range cookies {
		if cookies[i].Key == key {
			return &cookies[i].Value
		}
	}
	return nil
}

func (cookies *Cookies) Append(cookie ...Cookie) {
	for _, c := range cookie {
		if item := cookies.GetValue(c.Key); item != nil {
			*item = append(*item, c.Value...)
		} else {
			*cookies = append(*cookies, c)
		}
	}
}

func (cookies *Cookies) Parse(rawValues ...string) {
	for _, rawValue := range rawValues {
		cookies.Append(*ParseCookie(rawValue))
	}
}

func ParseCookie(rawValue string) *Cookie {
	parts := strings.SplitN(rawValue, "=", 2)
	if len(parts) == 1 {
		return &Cookie{Key: parts[0], Value: []string{""}}
	}
	return &Cookie{Key: parts[0], Value: []string{parts[1]}}
}
