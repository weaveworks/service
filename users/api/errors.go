package api

import (
	"net/http"

	common_render "github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users/render"
)

func renderError(w http.ResponseWriter, r *http.Request, err error) {
	common_render.Error(w, r, err, render.ErrorStatusCode)
}
