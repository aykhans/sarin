package types

import "strings"

type Param KeyValue[string, []string]

type Params []Param

func (params Params) GetValue(key string) *[]string {
	for i := range params {
		if params[i].Key == key {
			return &params[i].Value
		}
	}
	return nil
}

func (params *Params) Merge(param ...Param) {
	for _, p := range param {
		if item := params.GetValue(p.Key); item != nil {
			*item = append(*item, p.Value...)
		} else {
			*params = append(*params, p)
		}
	}
}

func (params *Params) Parse(rawValues ...string) {
	for _, rawValue := range rawValues {
		*params = append(*params, *ParseParam(rawValue))
	}
}

func ParseParam(rawValue string) *Param {
	parts := strings.SplitN(rawValue, "=", 2)
	if len(parts) == 1 {
		return &Param{Key: parts[0], Value: []string{""}}
	}
	return &Param{Key: parts[0], Value: []string{parts[1]}}
}
