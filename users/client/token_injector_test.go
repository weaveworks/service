package client

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitIntegration_GHIntegrationMiddleware(t *testing.T) {
	g := &mockGHIntegration{
		tok: "123",
	}
	m := GHIntegrationMiddleware{
		T: g,
	}
	req := http.Request{
		Header: http.Header{},
	}
	m.Wrap(&mockHTTPHandler{}).ServeHTTP(nil, &req)
	if req.Header.Get("GithubToken") == "" {
		t.Fatal("Did not set Github token")
	}
}

func TestGitIntegration_GHIntegrationMiddlewareFail(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	g := &mockGHIntegration{
		err: &APIError{
			StatusCode: http.StatusUnprocessableEntity,
			Status:     "422",
			Body:       "test",
		},
	}
	m := GHIntegrationMiddleware{
		T: g,
	}
	w := httptest.NewRecorder()
	m.Wrap(&mockHTTPHandler{}).ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Expecting 422 but got, %v", w.Code)
	}
}

type mockGHIntegration struct {
	tok string
	err error
}

func (g *mockGHIntegration) TokenForUser(_ *http.Request, _ string) (string, error) {
	return g.tok, g.err
}

type mockHTTPHandler struct {
	r *http.Request
}

func (h *mockHTTPHandler) ServeHTTP(_ http.ResponseWriter, req *http.Request) {
	h.r = req
}

func TestGitIntergration_TokenForuser(t *testing.T) {
	tripper := &mockRoundTripper{
		statusCode: 200,
		body:       `{"token": "123"}`,
	}
	tr := newRequester(tripper)

	tok, err := tr.TokenForUser(requestWithIDHeader("u1"), "github")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "123" {
		t.Fatalf("tok should have been 123, but got %v", tok)
	}
}

func TestGitIntegration_NoUserIdHeader(t *testing.T) {
	tr := newRequester(nil)
	_, err := tr.TokenForUser(requestWithIDHeader(""), "github")
	if err == nil {
		t.Fatalf("Expected error, but got: %v", err)
	}
}

func TestGitIntergration_EmptyToken(t *testing.T) {
	tripper := &mockRoundTripper{
		statusCode: 200,
		body:       `{"token": ""}`,
	}
	tr := newRequester(tripper)

	_, err := tr.TokenForUser(requestWithIDHeader("u1"), "github")
	if err == nil {
		t.Fatalf("Expecting empty token error, got %v", err)
	}
}

func TestGitIntergration_ErrorGettingToken(t *testing.T) {
	tripper := &mockRoundTripper{
		statusCode: 401,
	}
	tr := newRequester(tripper)

	_, err := tr.TokenForUser(requestWithIDHeader("u1"), "github")
	if err == nil {
		t.Fatalf("Expecting error after error getting token, got %v", err)
	}
}

type mockRoundTripper struct {
	statusCode int
	body       string
}

func (t *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.statusCode,
		Body:       ioutil.NopCloser(strings.NewReader(t.body)),
	}, nil
}

func newRequester(t http.RoundTripper) Integration {
	return &TokenRequester{
		client: http.Client{
			Transport: t,
		},
		UserIDHeader: "userID",
	}
}

func requestWithIDHeader(val string) *http.Request {
	h := http.Header{}
	h.Set("userID", val)
	return &http.Request{
		Header: h,
	}
}
