package transport

import "net/http"

// The RoundTripperFunc type is an adapter
// to allow the use of ordinary functions as RoundTripper.
// If f is a function with the appropriate signature,
// RoundTripperFunc(f) is a RoundTripper that calls f.
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip simply calls RoundTripperFunc
func (r RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return r(req)
}
