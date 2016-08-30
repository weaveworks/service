package templates

import (
	"bytes"
	"fmt"
	html "html/template"
	"io"
	"path/filepath"
	text "text/template"

	"github.com/Sirupsen/logrus"
)

// ErrTemplateNotFound is for when a template is not found. Usually this means
// the person deploying this messed up.
type ErrTemplateNotFound struct {
	name string
}

func (e ErrTemplateNotFound) Error() string {
	return fmt.Sprintf("Template Not Found: %s", e.name)
}

// Executor is something which we can execute like a template
type Executor interface {
	Execute(wr io.Writer, data interface{}) (err error)
}

// MustNewEngine creates a new Engine, or panics.
func MustNewEngine(dir string) Engine {
	h, err := html.ParseGlob(filepath.Join(dir, "*.html"))
	if err != nil {
		logrus.Fatal(err)
	}
	t, err := text.ParseGlob(filepath.Join(dir, "*.text"))
	if err != nil {
		logrus.Fatal(err)
	}

	return &extensionsTemplateEngine{h, t}
}

// Engine is a thing which loads and executes templates
type Engine interface {
	Lookup(name string) (Executor, error)
	Bytes(name string, data interface{}) ([]byte, error)
	QuietBytes(name string, data interface{}) []byte
}

type extensionsTemplateEngine struct {
	htmlTemplates *html.Template
	textTemplates *text.Template
}

// Lookup finds a new template with the given name
func (l *extensionsTemplateEngine) Lookup(name string) (t Executor, err error) {
	switch filepath.Ext(name) {
	case ".html":
		t = l.htmlTemplates.Lookup(name)
	case ".text":
		t = l.textTemplates.Lookup(name)
	}
	if t == nil {
		err = ErrTemplateNotFound{name}
	}
	return t, err
}

// Bytes finds and executes the given template by name
func (l *extensionsTemplateEngine) Bytes(templateName string, data interface{}) ([]byte, error) {
	t, err := l.Lookup(templateName)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = t.Execute(buf, data)
	return buf.Bytes(), err
}

// QuietBytes finds and executes the given template by name, but doesn't return errors
func (l *extensionsTemplateEngine) QuietBytes(name string, data interface{}) []byte {
	b, err := l.Bytes(name, data)
	if err != nil {
		logrus.Error(err)
	}
	return b
}
