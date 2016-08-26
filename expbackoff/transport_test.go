package expbackoff

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context/ctxhttp"

	"context"
)

type backoffTestServer struct {
	called       int
	lock         sync.Mutex
	_returnError bool
}

func (bt *backoffTestServer) getReturnError() bool {
	bt.lock.Lock()
	defer bt.lock.Unlock()
	return bt._returnError
}

func (bt *backoffTestServer) setReturnError(b bool) {
	bt.lock.Lock()
	bt._returnError = b
	bt.lock.Unlock()
}

func non200Error(r *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if r.StatusCode != 200 {
		return true
	}
	return false
}

func (bt *backoffTestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bt.called++
	if bt.getReturnError() {
		w.WriteHeader(500)
	}
}

func TestTransportSuccessOnTheFirst(t *testing.T) {
	bt := &backoffTestServer{}
	testServer := httptest.NewServer(bt)
	defer testServer.Close()

	transport := &Transport{
		Min:       10 * time.Millisecond,
		Max:       100 * time.Millisecond,
		Factor:    1.9,
		RetryFunc: non200Error,
	}
	client := &http.Client{Transport: transport}
	res, err := client.Get(testServer.URL)

	if res.StatusCode != 200 {
		t.Errorf("cannot get collect status code")
	}
	if err != nil {
		t.Errorf("error must be nil")
	}
	if bt.called != 1 {
		t.Errorf("called must be 1 actual %d", bt.called)
	}
}

func TestTransportSuccessOnTheThird(t *testing.T) {
	bt := &backoffTestServer{}
	testServer := httptest.NewServer(bt)
	defer testServer.Close()

	transport := &Transport{
		Min:       10 * time.Millisecond,
		Max:       400 * time.Millisecond,
		Factor:    1.9,
		RetryFunc: non200Error,
	}
	client := &http.Client{Transport: transport}

	var (
		res *http.Response
		err error
	)

	bt.setReturnError(true)

	go func() {
		time.Sleep(230 * time.Millisecond)
		bt.setReturnError(false)
	}()
	res, err = client.Get(testServer.URL)

	if res == nil || res.StatusCode != 200 {
		t.Errorf("cannot get collect status code")
	}
	if err != nil {
		t.Errorf("error must be nil")
	}
	if bt.called != 6 {
		t.Errorf("called must be 6 actual %d", bt.called)
	}
}

func TestTransportFailure(t *testing.T) {
	bt := &backoffTestServer{}
	testServer := httptest.NewServer(bt)
	defer testServer.Close()

	transport := &Transport{
		Min:       1 * time.Millisecond,
		Max:       3 * time.Millisecond,
		Factor:    1.9,
		RetryFunc: non200Error,
	}
	client := &http.Client{Transport: transport}

	var wg sync.WaitGroup

	var (
		res *http.Response
		err error
	)

	bt.setReturnError(true)
	wg.Add(2)
	go func() {
		res, err = client.Get(testServer.URL)
		wg.Done()
	}()
	go func() {
		time.Sleep(35 * time.Millisecond)
		bt.setReturnError(false)
		wg.Done()
	}()
	wg.Wait()

	if res == nil {
		t.Errorf("response must not be nil")
	}
	if res.StatusCode != 500 {
		t.Errorf("statusCode must be 500")
	}
	if err != nil {
		t.Errorf("error must be nil")
	}
	if bt.called != 3 {
		t.Errorf("called must be 3 actual %d", bt.called)
	}
}

func TestTransportCancel(t *testing.T) {
	bt := &backoffTestServer{}
	testServer := httptest.NewServer(bt)
	defer testServer.Close()

	transport := &Transport{
		Min:       50 * time.Millisecond,
		Max:       800 * time.Millisecond,
		Factor:    2,
		RetryFunc: non200Error,
	}

	client := &http.Client{Transport: transport}

	var (
		res *http.Response
		err error
	)

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		t.Fatalf("Failed to build request: %s", err.Error())
	}

	bt.setReturnError(true)
	wg := new(sync.WaitGroup)
	wg.Add(2)
	var duration time.Duration
	go func() {
		start := time.Now()
		res, err = client.Do(req.WithContext(ctx))
		end := time.Now()
		duration = end.Sub(start)
		wg.Done()
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
		wg.Done()
	}()

	wg.Wait()

	if duration < 90*time.Millisecond || 140*time.Millisecond < duration {
		t.Errorf("Failed to cancel. The request should be canceled in around 100ms, but %s taken", duration.String())
	}

	if res != nil {
		t.Error("Response should be nil")
	}

	if err == nil {
		t.Error("Error must not be nil")
	}

	if bt.called != 2 {
		t.Errorf("It should be called twice, but actual %d", bt.called)
	}
}

func TestTransportCancelWithCTXHTTP(t *testing.T) {
	bt := &backoffTestServer{}
	testServer := httptest.NewServer(bt)
	defer testServer.Close()

	transport := &Transport{
		Min:       50 * time.Millisecond,
		Max:       800 * time.Millisecond,
		Factor:    2,
		RetryFunc: non200Error,
	}

	client := &http.Client{Transport: transport}

	var (
		res *http.Response
		err error
	)

	ctx, cancel := context.WithCancel(context.Background())

	bt.setReturnError(true)
	wg := new(sync.WaitGroup)
	wg.Add(2)
	var duration time.Duration
	go func() {
		start := time.Now()
		res, err = ctxhttp.Get(ctx, client, testServer.URL)
		end := time.Now()
		duration = end.Sub(start)
		wg.Done()
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
		wg.Done()
	}()

	wg.Wait()

	if duration < 90*time.Millisecond || 140*time.Millisecond < duration {
		t.Errorf("Failed to cancel. The request should be canceled in around 100ms, but %s taken", duration.String())
	}

	if res != nil {
		t.Error("Response should be nil")
	}

	if err == nil {
		t.Error("Error must not be nil")
	}

	if bt.called != 2 {
		t.Errorf("It should be called twice, but actual %d", bt.called)
	}
}
