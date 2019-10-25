package limit

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

type httpResponseResult struct {
	res *http.Response
	err error
}

type requestPayload struct {
	responder func() (*http.Response, error)
	resCh     chan *httpResponseResult
}

type channelStarter func(chan struct{}) *priorityChannel

// RateLimit is an implementation of the RoundTripper
// that limits a quantity of requests in the groups
type RateLimit struct {
	Transport          http.RoundTripper
	GroupKeyFunc       func(r *http.Request) string
	PriorityHeaderName string
	Expire             *time.Duration
	channelStarter     channelStarter
	closeCh            chan struct{}
	channelMap         map[string]*priorityChannel
	ml                 *sync.Mutex
	initOnce           sync.Once
}

// ConstantGroupKeyFunc restricts whole requests in RateLimit
func ConstantGroupKeyFunc(r *http.Request) string {
	return "__constant_group_key__"
}

// DefaultPriorityHeaderName is the default name of a header to specify the priority of the request
const DefaultPriorityHeaderName = "X-Ratelimit-Priority"

func (t *RateLimit) priorityHeader() string {
	if t.PriorityHeaderName == "" {
		return DefaultPriorityHeaderName
	}
	return t.PriorityHeaderName
}

func (t *RateLimit) init() {
	if t.closeCh == nil {
		t.closeCh = make(chan struct{})
	}
	if t.ml == nil {
		t.ml = new(sync.Mutex)
	}
	if t.channelMap == nil {
		t.channelMap = map[string]*priorityChannel{}
	}
}

func (t *RateLimit) waitCh(key string) *priorityChannel {
	t.ml.Lock()
	defer t.ml.Unlock()
	ch, ok := t.channelMap[key]
	if ok {
		ch.add()
		return ch
	}
	ch = t.channelStarter(t.closeCh)
	t.channelMap[key] = ch
	ch.add()

	go func() {
		for {
			select {
			case <-t.closeCh:
				return
			case <-after(t.Expire):
			}
			if t.expire(ch, key) {
				break
			}
		}
	}()

	return ch
}

func (t *RateLimit) expire(ch *priorityChannel, key string) bool {
	t.ml.Lock()
	defer t.ml.Unlock()
	if ch.using() {
		return false
	}
	delete(t.channelMap, key)
	t.closeCh <- struct{}{}
	return true
}

func (t *RateLimit) distKey(req *http.Request) string {
	if t.GroupKeyFunc != nil {
		return t.GroupKeyFunc(req)
	}
	return GroupKeyByHost(req)
}

// RoundTrip implements the RoundTripper interface.
func (t *RateLimit) RoundTrip(req *http.Request) (*http.Response, error) {
	t.initOnce.Do(t.init)

	key := t.distKey(req)
	if key == "" {
		return t.transport().RoundTrip(req)
	}

	reqCh := t.waitCh(key)
	defer reqCh.done()
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
		}
	case <-t.closeCh:
		return nil, errors.New("request canceled")
	}
}

// CancelRequest cancels an in-flight request by closing its connection.
func (t *RateLimit) CancelRequest(req *http.Request) {
	type canceller interface {
		CancelRequest(*http.Request)
	}
	if c, ok := t.transport().(canceller); ok {
		c.CancelRequest(req)
	}
}

func (t *RateLimit) channelForRequest(pc *priorityChannel, r *http.Request) chan<- interface{} {
	pr := strings.ToLower(r.Header.Get(t.priorityHeader()))
	switch pr {
	case "high":
		return pc.High
	case "low":
		return pc.Low
	default:
		return pc.Normal
	}
}

func (t *RateLimit) transport() http.RoundTripper {
	it := t.Transport
	if it == nil {
		return http.DefaultTransport
	}
	return it
}

// Close will destruct itself
func (t *RateLimit) Close() {
	if t.closeCh != nil {
		close(t.closeCh)
	}
}

func after(d *time.Duration) <-chan time.Time {
	if d != nil {
		return time.After(*d)
	}
	var ch <-chan time.Time
	return ch
}
