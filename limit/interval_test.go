package limit

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type intervalTest struct {
	lastRequested time.Time
	minInterval   time.Duration
	l             sync.Mutex
}

func (it *intervalTest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt := time.Now()

	it.l.Lock()
	it.minInterval = rt.Sub(it.lastRequested)
	it.lastRequested = rt
	it.l.Unlock()
}

func TestInterval(t *testing.T) {
	it := &intervalTest{}
	s := httptest.NewServer(it)
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

	if it.minInterval < exInterval-(10*time.Millisecond) { // handler may delay
		t.Errorf("min request interval to server must grater than %s actual %s", exInterval.String(), it.minInterval.String())
	}
}

func TestIntervalFactory(t *testing.T) {
	it := &intervalTest{}
	s := httptest.NewServer(it)
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

	if it.minInterval < exInterval-(10*time.Millisecond) { // handler may delay
		t.Errorf("min request interval to server must grater than %s actual %s", exInterval.String(), it.minInterval.String())
	}
}

func TestIntervalWithExpire(t *testing.T) {
	it := &intervalTest{}
	s := httptest.NewServer(it)
	defer s.Close()

	exInterval := 100 * time.Millisecond

	transport := NewIntervalTransport(exInterval)
	transport.GroupKeyFunc = GroupKeyByHost
	expire := 10 * time.Millisecond
	transport.ExpireCheckInterval = &expire
	defer transport.Close()

	testClient := &http.Client{
		Transport: transport,
	}

	wg := sync.WaitGroup{}
	numReq := 10
	wg.Add(numReq)
	for i := 0; i < numReq; i++ {
		time.Sleep(20 * time.Millisecond)
		go func() {
			testClient.Get(s.URL)
			wg.Done()
		}()
	}
	wg.Wait()

	if it.minInterval < exInterval-(10*time.Millisecond) { // handler may delay
		t.Errorf("min request interval to server must grater than %s actual %s", exInterval.String(), it.minInterval.String())
	}

	time.Sleep(20 * time.Millisecond)

	{
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
	}
}
