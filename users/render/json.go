package render

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
)

// Time renders a timestamp into a string, in the format the user expects.
// Makes them nice and consistent.
func Time(t time.Time) string {
	utc := t.UTC()
	if utc.IsZero() {
		return ""
	}
	return utc.Format(time.RFC3339)
}

// JSON renders a response into the api as json.
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.Error(err)
	}
}
