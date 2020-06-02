package swagger2http

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/url"
	"strings"
)

const (
	swaggerVersion = "2.0"
)

type Swagger struct {
	// version
	Swagger  string   `json:"swagger"`
	Info     Info     `json:"info"`
	Host     string   `json:"host"`
	BasePath string   `json:"basePath"`
	Schemes  []string `json:"schemes"`
	Tags     []Tag    `json:"tags"`

	// lazy load field
	Paths               lazyRaw `json:"paths"`
	SecurityDefinitions lazyRaw `json:"securityDefinitions"`
	Definitions         lazyRaw `json:"definitions"`
	Parameters          lazyRaw `json:"parameters"`

	// caching
	refs       map[string]lazyRaw
	securities map[string]lazyRaw
}

func (s *Swagger) Load(file string) error {
	t, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	err = json.Unmarshal(t, s)
	if err == nil {
		return nil
	}
	if s.Swagger != swaggerVersion {
		return errInvalidSwaggerVersion
	}

	s.refs = make(map[string]lazyRaw)
	return nil
}

func (s *Swagger) Dump() (*bytes.Buffer, error) {
	var out = &bytes.Buffer{}
	err := s.writeHead(out)
	if err != nil {
		return nil, err
	}

	paths, err := s.getPaths()
	if err != nil {
		return nil, err
	}

	if len(paths) > 0 {
		err = s.loadRef()
		if err != nil {
			return nil, err
		}
	}
	for path := range paths {
		rs, err := s.unpackRequest(paths[path])
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			err = s.writeRequest(out, path, r)
			if err != nil {
				return nil, err
			}
		}
	}

	return out, err
}

func (s *Swagger) writeRequest(out *bytes.Buffer, path string, req Request) error {
	method := strings.ToUpper(req.Method)
	out.WriteString("###\n")
	out.WriteString("# " + strings.Join(req.Tags, ", ") + "." + req.OperationId + "\n")
	out.WriteString("# " + strings.ReplaceAll(req.Summary, "\n", "\n# ") + "\n")
	if req.Description != "" {
		out.WriteString("# " + strings.ReplaceAll(req.Description, "\n", "\n# ") + "\n")
	}

	queries := url.Values{}
	// ref here?
	for _, p := range req.Params {
		if p.In == "query" {
			val, err := p.GetDefault(s.refs, EFormatString)
			if err != nil {
				return err
			}
			queries.Add(p.Name, val)
		} else if p.In == "path" {
			val, err := p.GetDefault(s.refs, EFormatString)
			if err != nil {
				return err
			}
			path = strings.ReplaceAll(path, "{"+p.Name+"}", val)
		}
	}

	out.WriteString(method + " " + s.selectScheme() + "://" + s.Host + s.BasePath + path + "\n")
	if len(queries) > 0 {
		out.WriteString(" ?" + queries.Encode() + "\n")
	}
	for _, p := range req.Params {
		if p.Ref != "" {
			err := s.refs[p.Ref].UnpackTo(&p)
			if err != nil {
				return err
			}
		}
		if p.In == "header" {
			v := "_header_" + strings.ReplaceAll(p.Name, "-", "_")
			out.WriteString(p.Name + ": {{" + v + "}}\n")
		}
	}

	authKeys := make(map[string]string)
	for _, p := range req.Security {
		m, err := p.UnpackToMap()
		if err != nil {
			return err
		}
		for k := range m {
			sr, ok := s.securities[k]
			if !ok {
				continue
			}
			sp := Param{}
			err = sr.UnpackTo(&sp)
			if err != nil {
				return err
			}
			if sp.In == "header" {
				v := "_auth_" + strings.ReplaceAll(sp.Name, "-", "_")
				out.WriteString(sp.Name + ": {{" + v + "}}\n")
				authKeys[v] = sp.Name
			}
		}
	}

	out.WriteString("Content-Type: application/json\n")

	out.WriteByte('\n')
	for _, p := range req.Params {
		if p.Ref != "" {
			err := s.refs[p.Ref].UnpackTo(&p)
			if err != nil {
				return err
			}
		}
		if p.In == "body" {
			if p.Schema.Ref != "" {
				err := s.refs[p.Schema.Ref].UnpackTo(&p)
				if err != nil {
					return err
				}
			}

			val, err := p.GetDefault(s.refs, EFormatJSON)
			if err != nil {
				return err
			}
			pa := json.RawMessage(val)
			if pa == nil {
				pa = []byte("{}")
			}
			var dst bytes.Buffer
			_ = json.Indent(&dst, pa, "", "    ")

			out.WriteString(dst.String())
			out.WriteString("\n\n")
		}
	}
	out.WriteString("\n")
	return nil
}

func (s *Swagger) unpackRequest(raw lazyRaw) ([]Request, error) {
	reqs, err := raw.UnpackToMap()
	if err != nil {
		return nil, err
	}

	rs := []Request(nil)
	params := []Param(nil)
	for method := range reqs {
		if method == "parameters" {
			var ps []Param
			err = reqs[method].UnpackTo(&ps)
			if err != nil {
				return nil, err
			}
			params = append(params, ps...)
			continue
		}
		var r Request
		err = reqs[method].UnpackTo(&r)
		if err != nil {
			return nil, err
		}
		r.Params = append(params, r.Params...)
		r.Method = method
		rs = append(rs, r)
	}

	return rs, nil
}

func (s *Swagger) writeHead(out io.Writer) error {
	if s.Info.Title != "" {
		_, err := out.Write([]byte("# " + s.Info.Title + "\n"))
		if err != nil {
			return err
		}
	}
	if s.Info.Version != "" {
		_, err := out.Write([]byte("# " + s.Info.Version + "\n"))
		if err != nil {
			return err
		}
	}
	if s.Info.Description != "" {
		_, err := out.Write([]byte("# " + s.Info.Description + "\n"))
		if err != nil {
			return err
		}
	}
	_, err := out.Write([]byte("\n"))
	return err
}

func (s *Swagger) getPaths() (map[string]lazyRaw, error) {
	return s.Paths.UnpackToMap()
}

func (s *Swagger) loadRef() error {
	defs, err := s.Definitions.UnpackToMap()
	if err != nil {
		return err
	}
	s.refs = make(map[string]lazyRaw)
	keyPrefix := "#/definitions/"
	for r := range defs {
		s.refs[keyPrefix+r] = defs[r]
	}

	ps, err := s.Parameters.UnpackToMap()
	if err != nil {
		return err
	}

	keyPrefix = "#/parameters/"
	for r := range ps {
		s.refs[keyPrefix+r] = ps[r]
	}

	s.securities = make(map[string]lazyRaw)
	ss, err := s.SecurityDefinitions.UnpackToMap()
	for a := range ss {
		s.securities[a] = ss[a]
	}

	return nil
}

func (s *Swagger) selectScheme() string {
	if len(s.Schemes) > 0 {
		return s.Schemes[0]
	}
	return "http"
}
