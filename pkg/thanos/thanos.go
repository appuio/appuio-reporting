package thanos

import (
	"net/http"
	"strconv"
)

// PartialResponseRoundTripper adds a new RoundTripper to the chain that sets the partial_response query parameter to the value of Allow.
type PartialResponseRoundTripper struct {
	http.RoundTripper
	Allow bool
}

// AdditionalHeadersRoundTripper adds a new RoundTripper to the chain that sets additional static headers.
type AdditionalHeadersRoundTripper struct {
	http.RoundTripper
	Headers map[string][]string
}

// RoundTrip implements the RoundTripper interface.
func (t *PartialResponseRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query()
	q.Set("partial_response", strconv.FormatBool(t.Allow))
	req.URL.RawQuery = q.Encode()
	return t.RoundTripper.RoundTrip(req)
}

// RoundTrip implements the http.RoundTripper interface.
func (a *AdditionalHeadersRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// The specification of http.RoundTripper says that it shouldn't mutate
	// the request so make a copy of req.Header since this is all that is
	// modified.
	r2 := new(http.Request)
	*r2 = *r
	r2.Header = make(http.Header)
	for k, s := range r.Header {
		r2.Header[k] = s
	}
	for k, s := range a.Headers {
		r2.Header[k] = s
	}
	r = r2
	return a.RoundTripper.RoundTrip(r)
}
