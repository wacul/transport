package expbackoff

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"
)

// Transport is an implementation of the RoundTripper that retries a request
// with decreasing the rate on exponential backoff
type Transport struct {
	Transport http.RoundTripper
	Min       time.Duration
	Max       time.Duration

	// RetryFunc check the response of the Transport and decide whether to retry.
	RetryFunc func(*http.Response, error) bool
	Factor    float64
}

func (e *Transport) base() http.RoundTripper {
	if e.Transport == nil {
		return http.DefaultTransport
	}
	return e.Transport
}

func (e *Transport) shouldRetry(res *http.Response, err error) bool {
	if e.RetryFunc == nil {
		return err != nil
	}
	return e.RetryFunc(res, err)
}

// CancelRequest cancels an in-flight request by closing its connection.
func (e *Transport) CancelRequest(req *http.Request) {
	type canceller interface {
		CancelRequest(*http.Request)
	}
	if c, ok := e.base().(canceller); ok {
		c.CancelRequest(req)
	}
}

// RoundTrip implements the RoundTripper interface.
func (e *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqBytes []byte
	hasReqBody := req.Body != nil
	if hasReqBody {
		var err error
		reqBytes, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}

	current := e.Min
	for {
		// Copy the body for response readers.
		req.Body = ioutil.NopCloser(bytes.NewBuffer(reqBytes))
		res, err := e.base().RoundTrip(req)
		var resBytes []byte

		var retry bool
		if res != nil && res.Body != nil {
			bs, readErr := ioutil.ReadAll(res.Body)
			if readErr != nil {
				return nil, readErr
			}
			resBytes = bs
			res.Body = ioutil.NopCloser(bytes.NewBuffer(resBytes))
			retry = e.shouldRetry(res, err)
			res.Body = ioutil.NopCloser(bytes.NewBuffer(resBytes))
		} else {
			retry = e.shouldRetry(res, err)
		}

		if !retry || current >= e.Max {
			return res, err
		}

		time.Sleep(current)
		current = time.Duration(int64(float64(current) * e.Factor))
	}
}
