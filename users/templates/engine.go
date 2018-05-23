package templates

import (
	"bytes"
	"fmt"
	html "html/template"
	"io"
	"path/filepath"
	text "text/template"

	log "github.com/sirupsen/logrus"
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
		log.Fatal(err)
	}
	t, err := text.ParseGlob(filepath.Join(dir, "*.text"))
	if err != nil {
		log.Fatal(err)
	}

	return &extensionsTemplateEngine{h, t}
}

// Engine is a thing which loads and executes templates
type Engine interface {
	Lookup(name string) (Executor, error)
	Bytes(name string, data interface{}) ([]byte, error)
	QuietBytes(name string, data interface{}) []byte
	EmbedHTML(name string, wrapper string, title string, data interface{}) []byte
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
		// `t == nil` is going to be false. While the value inside the interface is `nil`,
		// the interface value itself is not. To compare this with nil we need to assert
		// its type which we know is `*html.Template`.
		if t.(*html.Template) == nil {
			t = nil
		}
	case ".text":
		t = l.textTemplates.Lookup(name)
		// See above.
		if t.(*text.Template) == nil {
			t = nil
		}
	}
	if t == nil {
		err = ErrTemplateNotFound{name}
	}
	return t, err
}

// Bytes finds and executes the given template by name
func (l *extensionsTemplateEngine) Bytes(templateName string, data interface{}) ([]byte, error) {
	t, err := l.Lookup(templateName)
	fmt.Println(t, err)
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
		log.Error(err)
	}
	return b
}

// Embed HTML content in a wrapper HTML page. This keeps emails looking consistent.
func (l *extensionsTemplateEngine) EmbedHTML(name, wrapper, title string, data interface{}) []byte {
	content, err := l.Bytes(name, data)

	if err != nil {
		log.Error(err)
		return nil
	}

	h, err := l.Bytes(wrapper, map[string]html.HTML{
		"Content": html.HTML(content),
		"Title":   html.HTML(title),
	})

	if err != nil {
		log.Error(err)
		return nil
	}

	return h
}
