package limit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type intervalTest struct {
	lastRequested time.Time
	intervals     []time.Duration
	l             sync.Mutex
}

func (handler *intervalTest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt := time.Now()

	handler.l.Lock()
	handler.intervals = append(handler.intervals, rt.Sub(handler.lastRequested))
	handler.lastRequested = rt
	handler.l.Unlock()
}

func TestIntervalWithOneGroupKey(t *testing.T) {
	handler := &intervalTest{
		lastRequested: time.Now(),
	}
	s := httptest.NewServer(handler)
	defer s.Close()

	exInterval := 100 * time.Millisecond

	transport := NewIntervalTransport(exInterval)
	transport.GroupKeyFunc = GroupKeyByHost
	defer transport.Close()

	testClient := &http.Client{
		Transport: transport,
	}

	wg := sync.WaitGroup{}
	numReq := 10
	wg.Add(numReq)
	for i := 0; i < numReq; i++ {
		go func() {
			testClient.Get(s.URL)
			wg.Done()
		}()
	}
	wg.Wait()

	for _, interval := range handler.intervals {
		if interval < exInterval-(10*time.Millisecond) { // handler may delay
			t.Errorf("min request interval to server must grater than %s actual %s", exInterval.String(), interval.String())
		}
	}
}

func TestIntervalFactory(t *testing.T) {
	handler := &intervalTest{}
	s := httptest.NewServer(handler)
	defer s.Close()
	exInterval := 100 * time.Millisecond
	factory := IntervalTransportFactory{
		Interval: exInterval,
	}

	wg := sync.WaitGroup{}
	numReq := 10
	wg.Add(numReq)
	for i := 0; i < numReq; i++ {
		go func() {
			transport := factory.NewTransport()
			transport.GroupKeyFunc = GroupKeyByHost
			testClient := &http.Client{
				Transport: transport,
			}

			testClient.Get(s.URL)
			wg.Done()
		}()
	}
	wg.Wait()

	for _, interval := range handler.intervals {
		if interval < exInterval-(10*time.Millisecond) { // handler may delay
			t.Errorf("min request interval to server must grater than %s actual %s", exInterval.String(), interval.String())
		}
	}
}

const groupKey = "x-key"

func groupKeyByHeader(r *http.Request) string {
	return r.Header.Get(groupKey)
}

type fakeCounterHandler struct {
	lastRequested map[string]time.Time
	intervals     map[string][]time.Duration
	l             sync.Mutex
}

func (handler *fakeCounterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt := time.Now()

	handler.l.Lock()
	key := r.Header.Get(groupKey)
	handler.intervals[key] = append(handler.intervals[key], rt.Sub(handler.lastRequested[key]))
	handler.lastRequested[key] = rt
	handler.l.Unlock()
}

func TestIntervalWithExpire(t *testing.T) {
	handler := &fakeCounterHandler{
		lastRequested: make(map[string]time.Time),
		intervals:     make(map[string][]time.Duration),
	}
	s := httptest.NewServer(handler)
	defer s.Close()

	exInterval := 100 * time.Millisecond

	transport := NewIntervalTransport(exInterval)
	transport.GroupKeyFunc = groupKeyByHeader
	expire := 300 * time.Millisecond
	transport.ExpireCheckInterval = &expire
	defer transport.Close()

	testClient := &http.Client{
		Transport: transport,
	}

	request := func(wg *sync.WaitGroup, key string) {
		req, err := http.NewRequest("GET", s.URL, nil)
		require.NoError(t, err)
		req.Header.Set(groupKey, key)
		go func() {
			testClient.Do(req)
			wg.Done()
		}()
	}

	numReq := 10
	keys := []string{"a", "b", "c"}

	// first
	{
		wg := sync.WaitGroup{}
		for i := 0; i < numReq; i++ {
			wg.Add(len(keys))
			for _, key := range keys {
				request(&wg, key)
			}
		}
		wg.Wait()
	}

	// waiting expire
	time.Sleep(300 * time.Millisecond)

	// second(create new channel after expire)
	{
		wg := sync.WaitGroup{}
		for i := 0; i < numReq; i++ {
			wg.Add(len(keys))
			for _, key := range keys {
				request(&wg, key)
			}
		}
		wg.Wait()
	}

	// assert all request intervals
	assertInterval := func(key string) {
		limit := exInterval - (10 * time.Millisecond) // handler may delay
		for _, interval := range handler.intervals[key] {
			if interval < limit {
				t.Errorf("min request interval to server must grater than %s actual %s", exInterval.String(), interval.String())
			}
		}
	}
	for _, key := range keys {
		assertInterval(key)
	}
}
