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
	extInterval := 50 * time.Millisecond
	interval := NewIntervalTransport(extInterval)
	interval.GroupKeyFunc = ConstantGroupKeyFunc
	interval.Transport = new(nilTransport)
	start := time.Now()

	req, reqErr := http.NewRequest("", "", nil)
	if reqErr != nil {
		t.Fatalf("Received unexpected error:\n%v", reqErr)
	}

	w := sync.WaitGroup{}
	w.Add(2)

	{
		exp := start.Add(extInterval)
		go func() {
			defer w.Done()
			_, err := interval.RoundTrip(req)
			if err != nil {
				t.Fatalf("Received unexpected error:\n%v", err)
			}
			act := time.Now()

			dt := exp.Sub(act)
			if dt < -extInterval || dt > extInterval {
				t.Fatalf("Max difference between %v and %v allowed is %v, but difference was %v in first response", exp, act, extInterval, dt)
			}
		}()
	}

	time.Sleep(100 * time.Microsecond) // to ensure request order

	{
		exp := start.Add(extInterval).Add(extInterval)
		go func() {
			defer w.Done()
			_, err := interval.RoundTrip(req)
			if err != nil {
				t.Fatalf("Received unexpected error:\n%v", err)
			}
			act := time.Now()

			dt := exp.Sub(act)
			if dt < -extInterval || dt > extInterval {
				t.Fatalf("Max difference between %v and %v allowed is %v, but difference was %v in second response", exp, act, extInterval, dt)
			}
		}()
	}

	w.Wait()
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
