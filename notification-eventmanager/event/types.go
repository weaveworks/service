package event

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users/templates"
	text "text/template"
	"time"
	"strings"
	"reflect"
)

type EventType interface {
	ReceiverData(recv string, data []byte) ReceiverData
}
type Types struct {
	engine templates.Engine
}

func NewEventTypes() Types {
	return Types{
		engine: templates.MustNewEngine("../templates/"),
	}
}

func (t Types) ReceiverData(recv string, e *types.Event) ReceiverData {
	fmt.Printf("FFS %v %#v\n", recv, e)
	switch e.Type {
	case "deploy":
		d := Deploy{}
		fmt.Println("WHUT")
		return d.ReceiverData(recv, e.Data)
	}

	return nil
}

func (t Types) renderTemplate(name string, e *types.Event, data map[string]interface{}) string {
	tmplname := fmt.Sprintf("%s.%s", e.Type, name)
	res, err := t.engine.Bytes(tmplname, data)
	if err != nil {
		res, err = json.Marshal(data)
		if err != nil {
			return "BOOM."
		}
	}
	return string(res)

}

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

func slackErrorAttachment(msg string) types.SlackAttachment {
	return types.SlackAttachment{
		Fallback: msg,
		Text:     msg,
		Color:    "warning",
	}
}

func slackResultAttachment(res update.Result) types.SlackAttachment {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "```")
	update.PrintResults(buf, res, 0)
	fmt.Fprintln(buf, "```")
	c := "good"
	if res.Error() != "" {
		c = "warning"
	}
	return types.SlackAttachment{
		Text:     buf.String(),
		MrkdwnIn: []string{"text"},
		Color:    c,
	}
}
