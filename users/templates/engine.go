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
func MustNewEngine(dirs ...string) Engine {
	var ht *html.Template
	var tt *text.Template
	var htmlfilenames, textfilenames []string

	htmlpattern := "*.html"
	textpattern := "*.text"

	for _, dir := range dirs {
		htmlfiles, err := filepath.Glob(filepath.Join(dir, htmlpattern))
		if err != nil {
			log.Fatal(err)
		}
		if len(htmlfiles) == 0 {
			log.Fatalf("html/template: pattern matches no files: %#q", filepath.Join(dir, htmlpattern))
		}
		htmlfilenames = append(htmlfilenames, htmlfiles...)

		textfiles, err := filepath.Glob(filepath.Join(dir, textpattern))
		if err != nil {
			log.Fatal(err)
		}
		if len(textfiles) == 0 {
			log.Fatalf("template: pattern matches no files: %#q", filepath.Join(dir, textpattern))
		}
		textfilenames = append(textfilenames, textfiles...)
	}

	h, err := html.ParseFiles(htmlfilenames...)
	if err != nil {
		log.Fatal(err)
	}
	ht = h

	t, err := text.ParseFiles(textfilenames...)
	if err != nil {
		log.Fatal(err)
	}
	tt = t

	return &extensionsTemplateEngine{ht, tt}
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
		if ht := l.htmlTemplates.Lookup(name); ht != nil {
			t = ht
		}
	case ".text":
		if tt := l.textTemplates.Lookup(name); tt != nil {
			t = tt
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
