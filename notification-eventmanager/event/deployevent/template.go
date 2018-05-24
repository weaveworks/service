package deployevent

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	text "text/template"
	"time"
)

var templateFuncs = text.FuncMap{
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

func textTemplate(tmplName, tmplStr string, args interface{}) (string, error) {
	tmpl, err := text.New(tmplName).Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return "", err
	}
	return buf.String(), nil
}
