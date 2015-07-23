package main

import (
	"bytes"
	"fmt"
	htmlTemplate "html/template"
	"io"
	"net/http"
	"path/filepath"
	textTemplate "text/template"

	"github.com/Sirupsen/logrus"
)

var (
	htmlTemplates *htmlTemplate.Template
	textTemplates *textTemplate.Template
)

type ErrTemplateNotFound struct {
	name string
}

func (e ErrTemplateNotFound) Error() string {
	return fmt.Sprintf("Template Not Found: %s", e.name)
}

type executor interface {
	Execute(wr io.Writer, data interface{}) (err error)
}

func loadTemplates() error {
	var err error
	htmlTemplates, err = htmlTemplate.ParseGlob("templates/*.html")
	if err != nil {
		return err
	}
	textTemplates, err = textTemplate.ParseGlob("templates/*.text")
	return err
}

func lookupTemplate(name string) (t executor, err error) {
	switch filepath.Ext(name) {
	case ".html":
		t = htmlTemplates.Lookup(name)
	case ".text":
		t = textTemplates.Lookup(name)
	}
	if t == nil {
		err = ErrTemplateNotFound{name}
	}
	return t, err
}

func executeTemplate(w http.ResponseWriter, templateName string, data interface{}) error {

	t, err := lookupTemplate(templateName)
	if err != nil {
		return err
	}

	return t.Execute(w, data)
}

func templateBytes(templateName string, data interface{}) ([]byte, error) {
	t, err := lookupTemplate(templateName)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = t.Execute(buf, data)
	return buf.Bytes(), err
}

func quietTemplateBytes(name string, data interface{}) []byte {
	b, err := templateBytes(name, data)
	if err != nil {
		logrus.Error(err)
	}
	return b
}
