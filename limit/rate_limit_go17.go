// +build go1.7

package limit

import (
	"context"
	"errors"
	"net/http"
)

// RoundTrip implements the RoundTripper interface.
func (t *RateLimit) RoundTrip(req *http.Request) (*http.Response, error) {
	t.initOnce.Do(t.init)

	key := t.distKey(req)
	if key == "" {
		return t.transport().RoundTrip(req)
	}

	canCh := req.Cancel
	ctx := req.Context()
	reqCh := t.waitCh(key)
	resCh := make(chan *httpResponseResult)
	mreq := requestPayload{
		responder: func() (*http.Response, error) {
			return t.transport().RoundTrip(req)
		},
		resCh: resCh,
	}

	select {
	case t.channelForRequest(reqCh, req) <- mreq:
		select {
		case mres := <-resCh:
			return mres.res, mres.err
		case <-t.closeCh:
			return nil, errors.New("request canceled")
		case <-canCh:
			return nil, context.Canceled
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-t.closeCh:
		return nil, errors.New("request canceled")
	case <-canCh:
		return nil, context.Canceled
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
