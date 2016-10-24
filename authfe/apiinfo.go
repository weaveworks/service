package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// apiInfo implements http.Handler to serve its contents as json
type apiInfo struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

func parseAPIInfo(info string) apiInfo {
	parts := strings.SplitN(info, ":", 2)
	if len(parts) < 2 {
		parts = append(parts, "unknown")
	}
	id := parts[0]
	version := parts[1]
	return apiInfo{id, version}
}

func (info apiInfo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(info)
	if err != nil {
		// should never happen
		panic(fmt.Sprintf("apiInfo struct not marshalable: %v", err))
	}
	w.Write(data)
}
