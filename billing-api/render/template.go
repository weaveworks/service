package render

import (
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Executor renders its data and sends it to a writer.
type Executor interface {
	Execute(wr io.Writer, data interface{}) error
}

// HTMLTemplate renders a template into the api with given data.
func HTMLTemplate(w http.ResponseWriter, status int, e Executor, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := e.Execute(w, data); err != nil {
		log.Error(err)
	}
}
