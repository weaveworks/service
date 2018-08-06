package render

import (
	"bytes"
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var funcMap = template.FuncMap{
	"toUpper":    strings.ToUpper,
	"iso8601":    func(t time.Time) string { return t.Format(time.RFC3339) },
	"join":       strings.Join,
	"replace":    strings.Replace,
	"trim":       strings.Trim,
	"trimLeft":   strings.TrimLeft,
	"trimPrefix": strings.TrimPrefix,
	"trimRight":  strings.TrimRight,
	"trimSuffix": strings.TrimSuffix,
	"trimSpace":  strings.TrimSpace,
	"last":       last,
}

func last(i int, a interface{}) (bool, error) {
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return i == v.Len()-1, nil
	}
	return false, fmt.Errorf("unsupported type: %T", a)
}

func executeTempl(tmpl string, data interface{}) (string, error) {
	t := template.Must(template.New(tmpl).Funcs(funcMap).Parse(tmpl))

	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return "", errors.Wrap(err, "cannot execute message")
	}

	return b.String(), nil
}
