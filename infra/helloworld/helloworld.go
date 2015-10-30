package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		fmt.Fprintf(w, "Hello world\n")
	})
	log.Printf("listening on :80")
	log.Fatal(http.ListenAndServe(":80", nil))
}
