// +build go1.7

package limit

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

type nilTransport struct{}

func (t *nilTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

func TestIntervalNotWaitCancelledRequest(t *testing.T) {
	extInterval := 50 * time.Millisecond
	interval := NewIntervalTransport(extInterval)
	interval.GroupKeyFunc = ConstantGroupKeyFunc
	interval.Transport = new(nilTransport)
	start := time.Now()

	req, reqErr := http.NewRequest("", "", nil)
	if reqErr != nil {
		t.Fatalf("Received unexpected error:\n%v", reqErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req = req.WithContext(ctx)

	w := sync.WaitGroup{}
	w.Add(2)

	{
		exp := start
		go func() {
			defer w.Done()
			_, err := interval.RoundTrip(req)
			if err != context.Canceled {
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
		exp := start
		go func() {
			defer w.Done()
			_, err := interval.RoundTrip(req)
			if err != context.Canceled {
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
