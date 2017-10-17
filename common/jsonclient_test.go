package common_test

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common"
	"mime/multipart"
	"bytes"
)

var ctx = context.Background()

func TestJSONClient_Get(t *testing.T) {
	ts, cl, reqfn := createMocks()
	defer ts.Close()

	resp := map[string]string{}
	err := cl.Get(ctx, "op", ts.URL, &resp)
	assert.NoError(t, err)

	assert.Equal(t, "yes", resp["response"])
	req, _ := reqfn()
	assert.Equal(t, "GET", req.Method)
}

func TestJSONClient_Post(t *testing.T) {
	ts, cl, reqfn := createMocks()
	defer ts.Close()

	data := map[string]string{
		"req": "post",
	}
	resp := map[string]string{}
	err := cl.Post(ctx, "op", ts.URL, data, &resp)
	assert.NoError(t, err)

	assert.Equal(t, "yes", resp["response"])
	req, body := reqfn()
	assert.Contains(t, body, `{"req":"post"}`)
	assert.Equal(t, "POST", req.Method)
}

func TestJSONClient_Put(t *testing.T) {
	ts, cl, reqfn := createMocks()
	defer ts.Close()

	data := map[string]string{
		"req": "put",
	}
	resp := map[string]string{}
	err := cl.Put(ctx, "op", ts.URL, data, &resp)
	assert.NoError(t, err)

	assert.Equal(t, "yes", resp["response"])
	req, body := reqfn()
	assert.Equal(t, "PUT", req.Method)
	assert.Contains(t, body, `{"req":"put"}`)
}

func TestJSONClient_Delete(t *testing.T) {
	ts, cl, reqfn := createMocks()
	defer ts.Close()

	resp := map[string]string{}
	err := cl.Delete(ctx, "op", ts.URL, &resp)
	assert.NoError(t, err)

	assert.Equal(t, "yes", resp["response"])
	req, _ := reqfn()
	assert.Equal(t, "DELETE", req.Method)
}

func TestJSONClient_Upload(t *testing.T) {
	ts, cl, reqfn := createMocks()
	defer ts.Close()

	content := bytes.NewBufferString("he he")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "billing-uploader.csv")
	assert.NoError(t, err)

	_, err = io.Copy(part, content)
	assert.NoError(t, err)

	err = writer.Close()
	assert.NoError(t, err)

	resp := map[string]string{}
	err = cl.Upload(ctx, "op", ts.URL, writer.FormDataContentType(), content, &resp)
	assert.NoError(t, err)

	assert.Equal(t, "yes", resp["response"])
	req, _ := reqfn()
	assert.Equal(t, "POST", req.Method)
}
func TestJSONClient_PostForm(t *testing.T) {
	ts, cl, reqfn := createMocks()
	defer ts.Close()

	resp := map[string]string{}
	data := url.Values{"req": []string{"postform"}}
	err := cl.PostForm(ctx, "foo", ts.URL, data, &resp)
	assert.NoError(t, err)

	assert.Equal(t, "yes", resp["response"])
	req, body := reqfn()
	assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
	assert.Contains(t, body, "req=postform")
	assert.Equal(t, "POST", req.Method)
}

func createMocks() (*httptest.Server, *common.JSONClient, func() (*http.Request, string)) {
	coll := instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{})
	cl := common.NewJSONClient(http.DefaultClient, coll)
	var req *http.Request
	var body string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.Body is closed as soon as we write to the ResponseWriter, let's save it
		bs, _ := ioutil.ReadAll(r.Body)
		body = string(bs)
		req = r
		io.WriteString(w, `{"response":"yes"}`)
	}))
	return ts, cl, func() (*http.Request, string) {
		return req, body
	}
}
