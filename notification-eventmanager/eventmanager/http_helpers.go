package eventmanager

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"

	"github.com/weaveworks/common/user"
)

// jsonWrapper wraps a function that takes the request and returns (some json, code, error),
// and writes an approriate response. If err is not nil, other values are ignored and 500 is returned.
type jsonWrapper struct {
	wrapped func(r *http.Request) (interface{}, int, error)
}

func (j jsonWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	result, code, err := j.wrapped(r)
	if err != nil {
		log.WithError(err).Error("Request error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	encoded := []byte{}
	if result != nil {
		encoded, err = json.Marshal(result)
		if err != nil {
			log.WithError(err).Error("Request response JSON encode error")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(code)
	w.Write(encoded)
}

// The following wrappers "with*" are adapters from implementation functions that take certain args,
// to a http.Handler. The wrappers parse the args and return any JSON + code returned from the function,
// or 500 if they error.

func withNoArgs(f func(*http.Request) (interface{}, int, error)) http.Handler {
	return jsonWrapper{func(r *http.Request) (interface{}, int, error) {
		result, code, err := f(r)
		return result, code, err
	}}
}

func withInstance(f func(*http.Request, string) (interface{}, int, error)) http.Handler {
	return jsonWrapper{func(r *http.Request) (interface{}, int, error) {
		instanceID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
		if err != nil {
			return nil, 0, err
		}
		result, code, err := f(r, instanceID)
		return result, code, err
	}}
}

func withID(f func(*http.Request, string) (interface{}, int, error)) http.Handler {
	return jsonWrapper{func(r *http.Request) (interface{}, int, error) {
		itemID, err := extractItemID(r)
		if err != nil {
			return nil, 0, err
		}
		result, code, err := f(r, itemID)
		return result, code, err
	}}
}

func withInstanceAndID(f func(*http.Request, string, string) (interface{}, int, error)) http.Handler {
	return jsonWrapper{func(r *http.Request) (interface{}, int, error) {
		instanceID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
		if err != nil {
			return nil, 0, err
		}
		itemID, err := extractItemID(r)
		if err != nil {
			return nil, 0, err
		}
		result, code, err := f(r, instanceID, itemID)
		return result, code, err
	}}
}

func extractItemID(r *http.Request) (string, error) {
	vars := mux.Vars(r)
	if id, ok := vars["id"]; ok {
		return id, nil
	}
	return "", fmt.Errorf("Missing expected path variable 'id'")
}

// reads the http body and unmarshals into given struct
func parseBody(r *http.Request, v interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, v)
	return err
}

// reads and parses the feature flags list from the request. always lowercase.
func getFeatureFlags(r *http.Request) []string {
	flagsStr := r.Header.Get("X-FeatureFlags")
	flags := []string{}
	if flagsStr == "" {
		return flags
	}
	for _, flag := range strings.Split(flagsStr, " ") {
		flags = append(flags, strings.ToLower(flag))
	}
	return flags
}

// isStatusErrorCode returns true if the error has the given status code.
func isStatusErrorCode(err error, code int) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return code == int(st.Code())
}
