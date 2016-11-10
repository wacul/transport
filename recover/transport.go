package recover

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

// Transport is an implementation of the RoundTripper.
// That uses the Base and Spare alternatively if the first one gets the error.
type Transport struct {
	Base  http.RoundTripper
	Spare http.RoundTripper

	// UseSpareFunc check the response of the Base and decide whether to use spare.
	UseSpareFunc func(*http.Response, error) bool
}

func (f *Transport) base() http.RoundTripper {
	if f.Base == nil {
		return http.DefaultTransport
	}
	return f.Base
}

func (f *Transport) spare() http.RoundTripper {
	if f.Spare == nil {
		return http.DefaultTransport
	}
	return f.Spare
}

// CancelRequest cancels an in-flight request by closing its connection.
func (f *Transport) CancelRequest(req *http.Request) {
	type canceller interface {
		CancelRequest(*http.Request)
	}
	if c, ok := f.base().(canceller); ok {
		c.CancelRequest(req)
	}
	if c, ok := f.spare().(canceller); ok {
		c.CancelRequest(req)
	}
}

func (f *Transport) useSpare(res *http.Response, err error) bool {
	if f.UseSpareFunc == nil {
		return err != nil
	}
	return f.UseSpareFunc(res, err)
}

// RoundTrip implements the RoundTripper interface.
func (f *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var rb []byte
	hasBody := req.Body != nil
	if hasBody {
		_rb, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		rb = _rb
		req.Body.Close()
		req.Body = ioutil.NopCloser(bytes.NewBuffer(rb))
	}

	res, err := f.base().RoundTrip(req)
	if f.useSpare(res, err) {
		if hasBody {
			req.Body = ioutil.NopCloser(bytes.NewBuffer(rb))
		}
		return f.spare().RoundTrip(req)
	}
	return res, err
}
