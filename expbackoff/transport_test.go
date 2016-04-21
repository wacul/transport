package expbackoff

import (
	"sync"
	"testing"
	"time"
)
import (
	"net/http"
	"net/http/httptest"
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

func TestExponentialBackoffTransport(t *testing.T) {
	// 単発成功
	{
		bt := &backoffTestServer{}
		testServer := httptest.NewServer(bt)
		defer testServer.Close()

		eb := &Transport{
			Min:       10 * time.Millisecond,
			Max:       100 * time.Millisecond,
			Factor:    1.9,
			RetryFunc: non200Error,
		}
		c := &http.Client{Transport: eb}
		res, err := c.Get(testServer.URL)

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

	// ２回バックオフで成功
	{
		bt := &backoffTestServer{}
		testServer := httptest.NewServer(bt)
		defer testServer.Close()

		eb := &Transport{
			Min:       10 * time.Millisecond,
			Max:       400 * time.Millisecond,
			Factor:    1.9,
			RetryFunc: non200Error,
		}
		c := &http.Client{Transport: eb}

		var (
			res *http.Response
			err error
		)

		bt.setReturnError(true)

		go func() {
			time.Sleep(230 * time.Millisecond)
			bt.setReturnError(false)
		}()
		res, err = c.Get(testServer.URL)

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

	// maxまで待ってエラー
	{
		bt := &backoffTestServer{}
		testServer := httptest.NewServer(bt)
		defer testServer.Close()

		eb := &Transport{
			Min:       1 * time.Millisecond,
			Max:       3 * time.Millisecond,
			Factor:    1.9,
			RetryFunc: non200Error,
		}
		c := &http.Client{Transport: eb}

		var wg sync.WaitGroup

		var (
			res *http.Response
			err error
		)

		bt.setReturnError(true)
		wg.Add(2)
		go func() {
			res, err = c.Get(testServer.URL)
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
}
