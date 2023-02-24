package thanos

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPartialResponseRoundTripper_X(t *testing.T) {
	testCases := []struct {
		url   string
		allow bool
	}{
		{
			url:   "https://thanos.io",
			allow: false,
		},
		{
			url:   "https://thanos.io?testly=blub",
			allow: false,
		},
		{
			url:   "https://thanos.io",
			allow: true,
		},
		{
			url:   "https://thanos.io?testly=blub",
			allow: true,
		},
	}
	for _, tC := range testCases {
		t.Run(fmt.Sprintf("allow %v, url %s", tC.allow, tC.url), func(t *testing.T) {
			origUrl, err := url.Parse(tC.url)
			require.NoError(t, err)

			rt := PartialResponseRoundTripper{
				RoundTripper: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					require.Contains(t, r.URL.RawQuery, `partial_response=`+strconv.FormatBool(tC.allow))
					require.Contains(t, r.URL.RawQuery, origUrl.RawQuery)
					return nil, errors.New("not implemented")
				}),
				Allow: tC.allow,
			}

			_, _ = rt.RoundTrip(httptest.NewRequest("GET", tC.url, nil))
		})
	}
}

func TestAdditionalHeadersResponseRoundTripper_X(t *testing.T) {
	testCases := []struct {
		url     string
		headers map[string][]string
	}{
		{
			url: "https://thanos.io",
			headers: map[string][]string{
				"X-Test-Header": []string{"foobar"},
			},
		},
		{
			url:     "https://thanos.io?testly=blub",
			headers: map[string][]string{},
		},
		{
			url: "https://thanos.io",
			headers: map[string][]string{
				"X-Test-One": []string{"one"},
				"X-Test-Two": []string{"two"},
			},
		},
		{
			url: "https://thanos.io?testly=blub",
			headers: map[string][]string{
				"X-Test-One": []string{"one", "two", "three"},
				"X-Test-Two": []string{"two"},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(fmt.Sprintf("headers %v, url %s", tC.headers, tC.url), func(t *testing.T) {
			rt := AdditionalHeadersRoundTripper{
				RoundTripper: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					for k, s := range tC.headers {
						require.Equal(t, r.Header.Values(k), s)
					}
					return nil, errors.New("not implemented")
				}),
				Headers: tC.headers,
			}

			_, _ = rt.RoundTrip(httptest.NewRequest("GET", tC.url, nil))
		})
	}
}

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (s roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return s(r)
}
