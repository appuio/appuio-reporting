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

// RoundTrip implements the RoundTripper interface.
func (t *PartialResponseRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query()
	q.Set("partial_response", strconv.FormatBool(t.Allow))
	req.URL.RawQuery = q.Encode()
	return t.RoundTripper.RoundTrip(req)
}
