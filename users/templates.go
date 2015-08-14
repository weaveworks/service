package main

import (
	"bytes"
	"fmt"
	html "html/template"
	"io"
	"path/filepath"
	text "text/template"

	"github.com/Sirupsen/logrus"
)

type errTemplateNotFound struct {
	name string
}

func (e errTemplateNotFound) Error() string {
	return fmt.Sprintf("Template Not Found: %s", e.name)
}

type executor interface {
	Execute(wr io.Writer, data interface{}) (err error)
}

func setupTemplates() *extensionTemplateEngine {
	h, err := html.ParseGlob("templates/*.html")
	if err != nil {
		logrus.Fatal(err)
	}
	t, err := text.ParseGlob("templates/*.text")
	if err != nil {
		logrus.Fatal(err)
	}

	return &extensionTemplateEngine{h, t}
}

type templateEngine interface {
	lookup(name string) (executor, error)
	bytes(name string, data interface{}) ([]byte, error)
	quietBytes(name string, data interface{}) []byte
}

type extensionTemplateEngine struct {
	htmlTemplates *html.Template
	textTemplates *text.Template
}

func (l *extensionTemplateEngine) lookup(name string) (t executor, err error) {
	switch filepath.Ext(name) {
	case ".html":
		t = l.htmlTemplates.Lookup(name)
	case ".text":
		t = l.textTemplates.Lookup(name)
	}
	if t == nil {
		err = errTemplateNotFound{name}
	}
	return t, err
}

func (l *extensionTemplateEngine) bytes(templateName string, data interface{}) ([]byte, error) {
	t, err := l.lookup(templateName)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = t.Execute(buf, data)
	return buf.Bytes(), err
}

func (l *extensionTemplateEngine) quietBytes(name string, data interface{}) []byte {
	b, err := l.bytes(name, data)
	if err != nil {
		logrus.Error(err)
	}
	return b
}
