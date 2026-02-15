package types

import "strings"

type Header KeyValue[string, []string]

type Headers []Header

func (headers Headers) Has(key string) bool {
	for i := range headers {
		if headers[i].Key == key {
			return true
		}
	}
	return false
}

func (headers Headers) GetValue(key string) *[]string {
	for i := range headers {
		if headers[i].Key == key {
			return &headers[i].Value
		}
	}
	return nil
}

func (headers *Headers) Merge(header ...Header) {
	for _, h := range header {
		if item := headers.GetValue(h.Key); item != nil {
			*item = append(*item, h.Value...)
		} else {
			*headers = append(*headers, h)
		}
	}
}

func (headers *Headers) Parse(rawValues ...string) {
	for _, rawValue := range rawValues {
		*headers = append(*headers, *ParseHeader(rawValue))
	}
}

func ParseHeader(rawValue string) *Header {
	parts := strings.SplitN(rawValue, ": ", 2)
	if len(parts) == 1 {
		return &Header{Key: parts[0], Value: []string{""}}
	}
	return &Header{Key: parts[0], Value: []string{parts[1]}}
}
