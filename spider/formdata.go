package spider

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
)

type FormData struct {
	m map[string]string
}

func NewFormData() *FormData {
	return &FormData{m: make(map[string]string)}
}

func (f *FormData) Add(k, v string) *FormData {
	f.m[k] = v
	return f
}
func (f *FormData) Del(k string) *FormData {
	delete(f.m, k)
	return f
}

func (f *FormData) Encode() io.Reader {
	var buf bytes.Buffer
	for k, v := range f.m {
		buf.WriteString(fmt.Sprintf("%s=%s&", url.QueryEscape(k), url.QueryEscape(v)))
	}
	buf.Truncate(buf.Len() - 1)
	return &buf
}

func (f *FormData) Each(fn func(k, v string)) {
	for k, v := range f.m {
		fn(k, v)
	}
}
