// +build !integration

package main

import (
	"html/template"
	"testing"
)

func setup(t *testing.T) {
	users = make(map[string]*User)

	if templates == nil {
		var err error
		templates, err = template.ParseGlob("templates/*.html")
		if err != nil {
			t.Fatal(err)
		}
	}
}

func cleanup(t *testing.T) {
}
