package limit

import (
	"sync"
	"time"
)

// IntervalTransportFactory manages the RoundTripper
// that cooperate with the each of themselves to limit intervals of the requests
type IntervalTransportFactory struct {
	Interval   time.Duration
	Expire     *time.Duration
	closeCh    chan struct{}
	channelMap map[string]*priorityChannel
	ml         *sync.Mutex
	initOnce   sync.Once
}

func (f *IntervalTransportFactory) init() {
	if f.closeCh == nil {
		f.closeCh = make(chan struct{})
	}
	if f.ml == nil {
		f.ml = new(sync.Mutex)
	}
	if f.channelMap == nil {
		f.channelMap = map[string]*priorityChannel{}
	}
}

// NewTransport generates the RoundTripper
// that cooperate with the each of themselves to limit intervals of the requests
func (f *IntervalTransportFactory) NewTransport() *RateLimit {
	f.initOnce.Do(f.init)
	return &RateLimit{
		channelStarter: getIntervalStarter(f.Interval, f.Expire),
		closeCh:        f.closeCh,
		channelMap:     f.channelMap,
		ml:             f.ml,
	}
}

// NewIntervalTransport generates the RoundTripper
// that limits intervals of the requests in the groups
func NewIntervalTransport(interval time.Duration, expire *time.Duration) *RateLimit {
	return &RateLimit{
		channelStarter: getIntervalStarter(interval, expire),
	}
}

func getIntervalStarter(interval time.Duration, expire *time.Duration) channelStarter {
	return func(closeCh chan struct{}, expireCh chan<- struct{}) *priorityChannel {
		pc := initPriorityChannel(closeCh)
		tick := time.Tick(interval)

		go func() {
			for {
				select {
				case <-closeCh:
					return
				case <-tick:
					select {
					case <-closeCh:
						return
					case iReq := <-pc.Out:
						go func() {
							req := iReq.(requestPayload)
							res := &httpResponseResult{}
							res.res, res.err = req.responder()
							select {
							case <-closeCh:
								return
							case req.resCh <- res:
							}
						}()
					case <-after(expire):
						expireCh <- struct{}{}
					}
				}
			}
		}()

		return pc
	}
}

func after(d *time.Duration) <-chan time.Time {
	if d != nil {
		return time.After(*d)
	}
	return make(<-chan time.Time)
}
