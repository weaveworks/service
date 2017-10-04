package zuora

import (
	"context"
	"io"
	"net/http"

	"github.com/weaveworks/common/logging"
)

const (
	filesPath = "files/%s"
)

// ServeFile writes out a file that's stored in Zuora.
func (z *Zuora) ServeFile(ctx context.Context, w http.ResponseWriter, fileID string) {
	logger := logging.With(ctx)
	url := z.URL(filesPath, fileID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	req = req.WithContext(ctx)
	req.SetBasicAuth(z.cfg.Username, z.cfg.Password)
	logger.Debugf("File request: %+v", req)

	resp, err := z.do(ctx, filesPath, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	logger.Debugf("Invoice response: %+v", resp)

	if resp.StatusCode/100 != 2 {
		http.Error(w, resp.Status, resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Content-Disposition", resp.Header.Get("Content-Disposition"))
	w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))

	if _, err := io.Copy(w, resp.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
