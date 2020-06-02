package swagger2http

import (
	"encoding/json"
	"errors"
)

const (
	ETypeObject  = "object"
	ETypeString  = "string"
	ETypeBoolean = "boolean"
	ETypeArray   = "array"
	ETypeInteger = "integer"

	EFormatJSON   = "json"
	EFormatString = "string"
)

type Info struct {
	Version     string `json:"version"`
	Description string `json:"description"`
	Title       string `json:"title"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Param struct {
	In       string
	Type     string
	Name     string
	Default  string
	Required lazyRaw
	Schema   struct {
		Ref string `json:"$ref"`
	}
	Ref string `json:"$ref"`

	Format string

	Enum  []string
	Items struct {
		Ref     string `json:"$ref"`
		Title   string
		Type    string
		Enum    []string
		Default string
	}
	Example string

	Properties lazyRaw
}

func (p Param) GetDefault(refs map[string]lazyRaw, format string) (ret string, err error) {
	defer func() {
		if format != EFormatJSON || ret == "" {
			return
		}
		// json
		switch p.Type {
		case ETypeArray:
			ret = "[\"" + ret + "\"]"
			break
		case ETypeString:
			ret = "\"" + ret + "\""
			break
		case EFormatJSON:
			ret = "<OBJECT>"
		default:
		}
	}()
	if p.Default != "" {
		return p.Default, nil
	}
	if p.Ref != "" {
		var np Param
		err := refs[p.Ref].UnpackTo(&np)
		if err != nil {
			return "", nil
		}
		return np.GetDefault(refs, format)
	}
	if p.Schema.Ref != "" {
		var np Param
		err := refs[p.Schema.Ref].UnpackTo(&np)
		if err != nil {
			return "", nil
		}
		return np.GetDefault(refs, format)
	}
	switch p.Type {
	case ETypeString:
		if p.Format == "int64" {
			return "0", nil
		}
		if len(p.Enum) > 0 {
			return p.Enum[0], nil
		}
		if p.Example != "" {
			return p.Example, nil
		}
		return ETypeString, nil
	case ETypeBoolean:
		return "false", nil
	case ETypeInteger:
		return "0", nil
	case ETypeArray:
		if p.Items.Default != "" {
			return p.Items.Default, nil
		}
		if len(p.Items.Enum) > 0 {
			return p.Items.Enum[0], nil
		}
		if p.Items.Type == ETypeInteger {
			return "0", nil
		}
		return ETypeString, nil
	case ETypeObject:
		n := make(map[string]json.RawMessage)
		if p.Properties == nil {
			return "{}", nil
		}
		m, err := p.Properties.UnpackToMap()
		if err != nil {
			return "", err
		}
		for k, raw := range m {
			var np Param
			err = raw.UnpackTo(&np)
			if err != nil {
				return "", err
			}
			child, err := np.GetDefault(refs, format)
			if err != nil {
				return "", err
			}
			n[k] = json.RawMessage(child)
		}
		nb, err := json.Marshal(n)
		return string(nb), err
	default:
		return "", nil
	}
}

type Request struct {
	Method      string
	Tags        []string
	Summary     string
	Description string
	OperationId string `json:"operationId"`
	Produces    []string
	Params      []Param `json:"parameters"`
	Responses   lazyRaw
	Security    []lazyRaw
}

type lazyRaw json.RawMessage

func (l lazyRaw) UnpackToMap() (map[string]lazyRaw, error) {
	if l == nil {
		return nil, nil
	}
	var child map[string]lazyRaw
	err := json.Unmarshal(l, &child)
	return child, err
}

func (l lazyRaw) UnpackTo(v interface{}) error {
	if l == nil {
		return nil
	}
	err := json.Unmarshal(l, v)
	return err
}

// MarshalJSON returns m as the JSON encoding of m.
func (l lazyRaw) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("null"), nil
	}
	return l, nil
}

// UnmarshalJSON sets *m to a copy of data.
func (l *lazyRaw) UnmarshalJSON(data []byte) error {
	if l == nil {
		return errors.New("json.RawMessage: UnmarshalJSON on nil pointer")
	}
	*l = append((*l)[0:0], data...)
	return nil
}

var _ json.Marshaler = (*lazyRaw)(nil)
var _ json.Unmarshaler = (*lazyRaw)(nil)
