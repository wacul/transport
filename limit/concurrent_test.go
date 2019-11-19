package limit

import (
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type concurrentTest struct {
	currentReq       int
	maxConcurrentReq int
	l                sync.Mutex
}

func (ct *concurrentTest) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ct.l.Lock()
	ct.currentReq++
	ct.l.Unlock()

	time.Sleep(1 * time.Millisecond)

	ct.l.Lock()
	ct.maxConcurrentReq = int(math.Max(float64(ct.currentReq), float64(ct.maxConcurrentReq)))
	ct.l.Unlock()

	time.Sleep(1 * time.Millisecond)

	ct.l.Lock()
	ct.currentReq--
	ct.l.Unlock()
}

func TestConcurrent(t *testing.T) {
	ct := &concurrentTest{}
	s := httptest.NewServer(ct)
	defer s.Close()

	tr := NewMaxConcurrentTransport(10)
	tr.GroupKeyFunc = GroupKeyByHost

	testClient := &http.Client{
		Transport: tr,
	}

	wg := sync.WaitGroup{}
	numReq := 100
	wg.Add(numReq)
	for i := 0; i < numReq; i++ {
		go func() {
			testClient.Get(s.URL)
			wg.Done()
		}()
	}
	wg.Wait()

	if ct.maxConcurrentReq > 10 {
		t.Errorf("max concurrent request to server should less than %d, actual %d", 10, ct.maxConcurrentReq)
	}
}

func TestConcurrentFactory(t *testing.T) {
	ct := &concurrentTest{}
	s := httptest.NewServer(ct)
	defer s.Close()

	factory := MaxConcurrentTransportFactory{
		MaxConcurrent: 10,
	}

	wg := sync.WaitGroup{}
	numReq := 100
	wg.Add(numReq)

	for i := 0; i < numReq; i++ {
		go func() {
			tr := factory.NewTransport()
			tr.GroupKeyFunc = GroupKeyByHost
			testClient := &http.Client{
				Transport: tr,
			}

			testClient.Get(s.URL)
			wg.Done()
		}()
	}
	wg.Wait()

	if ct.maxConcurrentReq > 10 {
		t.Errorf("max concurrent request to server should less than %d, actual %d", 10, ct.maxConcurrentReq)
	}
}
