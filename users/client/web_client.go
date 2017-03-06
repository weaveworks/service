package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/tokens"
)

type webClient struct {
	url    string
	client *http.Client
}

func newWebClient(url string) *webClient {
	return &webClient{
		url: url,
		client: &http.Client{
			Transport: &nethttp.Transport{
				RoundTripper: &http.Transport{
					// Rest are from http.DefaultTransport
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:   30 * time.Second,
						KeepAlive: 30 * time.Second,
					}).DialContext,
					TLSHandshakeTimeout:   10 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
			},
		},
	}
}

// LookupOrg authenticates a cookie for access to an org by extenal ID.
func (m *webClient) LookupOrg(ctx context.Context, in *users.LookupOrgRequest, opts ...grpc.CallOption) (*users.LookupOrgResponse, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/private/api/users/lookup/%s", m.url, url.QueryEscape(in.OrgExternalID)), nil)
	if err != nil {
		return nil, err
	}
	request.AddCookie(&http.Cookie{
		Name:  AuthCookieName,
		Value: in.Cookie,
	})
	request = request.WithContext(ctx)

	response := &users.LookupOrgResponse{}
	return response, m.doRequest(request, response)
}

// LookupUsingToken authenticates a token for access to an org.
func (m *webClient) LookupUsingToken(ctx context.Context, in *users.LookupUsingTokenRequest, opts ...grpc.CallOption) (*users.LookupUsingTokenResponse, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/private/api/users/lookup", m.url), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set(tokens.AuthHeaderName, tokens.Prefix+in.Token)
	request = request.WithContext(ctx)

	response := &users.LookupUsingTokenResponse{}
	return response, m.doRequest(request, response)
}

// LookupAdmin authenticates a cookie for admin access.
func (m *webClient) LookupAdmin(ctx context.Context, in *users.LookupAdminRequest, opts ...grpc.CallOption) (*users.LookupAdminResponse, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/private/api/users/admin", m.url), nil)
	if err != nil {
		return nil, err
	}
	request.AddCookie(&http.Cookie{
		Name:  AuthCookieName,
		Value: in.Cookie,
	})
	request = request.WithContext(ctx)

	response := &users.LookupAdminResponse{}
	return response, m.doRequest(request, response)
}

// LookupUser authenticates a cookie.
func (m *webClient) LookupUser(ctx context.Context, in *users.LookupUserRequest, opts ...grpc.CallOption) (*users.LookupUserResponse, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/private/api/users/lookup_user", m.url), nil)
	if err != nil {
		return nil, err
	}
	request.AddCookie(&http.Cookie{
		Name:  AuthCookieName,
		Value: in.Cookie,
	})
	request = request.WithContext(ctx)

	response := &users.LookupUserResponse{}
	return response, m.doRequest(request, response)
}

func (m *webClient) doRequest(r *http.Request, response interface{}) error {
	var ht *nethttp.Tracer
	r, ht = nethttp.TraceRequest(opentracing.GlobalTracer(), r)
	defer ht.Finish()

	// Contact the authorization server
	res, err := m.client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Parse the response
	if res.StatusCode != http.StatusOK {
		return &Unauthorized{res.StatusCode}
	}

	return json.NewDecoder(res.Body).Decode(response)
}
