package expbackoff

import (
	"bytes"
	"context"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"
)

// Transport is an implementation of the RoundTripper that retries a request
// with decreasing the rate on exponential backoff
type Transport struct {
	Transport       http.RoundTripper
	Min             time.Duration
	Max             time.Duration
	RandomizeFactor float64

	// RetryFunc check the response of the Transport and decide whether to retry.
	RetryFunc func(*http.Response, error) bool
	Factor    float64
}

func (t *Transport) base() http.RoundTripper {
	if t.Transport == nil {
		return http.DefaultTransport
	}
	return t.Transport
}

func (t *Transport) shouldRetry(res *http.Response, err error) bool {
	if t.RetryFunc == nil {
		return err != nil
	}
	return t.RetryFunc(res, err)
}

// RoundTrip implements the RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqBytes []byte
	hasReqBody := req.Body != nil
	if hasReqBody {
		var err error
		reqBytes, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}

	current := t.Min
	for {
		// Copy the body for response readers.
		req.Body = ioutil.NopCloser(bytes.NewBuffer(reqBytes))
		res, err := t.base().RoundTrip(req)
		var resBytes []byte

		var retry bool
		if res != nil && res.Body != nil {
			bs, readErr := ioutil.ReadAll(res.Body)
			if readErr != nil {
				return nil, readErr
			}
			resBytes = bs
			res.Body = ioutil.NopCloser(bytes.NewBuffer(resBytes))
			retry = t.shouldRetry(res, err)
			res.Body = ioutil.NopCloser(bytes.NewBuffer(resBytes))
		} else {
			retry = t.shouldRetry(res, err)
		}

		if !retry || current >= t.Max {
			return res, err
		}

		select {
		case <-req.Context().Done():
			return nil, context.Canceled
		case <-time.After(current):
		}
		current = t.nextWait(current)
	}
}

func (t *Transport) nextWait(current time.Duration) time.Duration {
	r := 1.0
	if t.RandomizeFactor > 0 {
		f := t.RandomizeFactor
		if f > 1 {
			f = 1
		}
		r = (rand.Float64()-0.5)*2*f + 1
	}
	return time.Duration(int64(float64(current) * t.Factor * r))
}
