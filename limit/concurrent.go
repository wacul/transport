package limit

import "sync"

// MaxConcurrentTransportFactory manages the RoundTripper
// that cooperate with the each of themselves to limit concurrency of the requests
type MaxConcurrentTransportFactory struct {
	MaxConcurrent int
	closeCh       chan struct{}
	channelMap    map[string]*priorityChannel
	ml            *sync.Mutex
	initOnce      sync.Once
}

func (f *MaxConcurrentTransportFactory) init() {
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
// that cooperate with the each of themselves to limit concurrency of the requests
func (f *MaxConcurrentTransportFactory) NewTransport() *RateLimit {
	f.initOnce.Do(f.init)
	return &RateLimit{
		channelStarter: getConcurrentStarter(f.MaxConcurrent),
		closeCh:        f.closeCh,
		channelMap:     f.channelMap,
		ml:             f.ml,
	}
}

// NewMaxConcurrentTransport generates the RoundTripper
// that limits concurrency of the requests in the groups
func NewMaxConcurrentTransport(concurrent int) *RateLimit {
	return &RateLimit{
		channelStarter: getConcurrentStarter(concurrent),
	}
}

func getConcurrentStarter(num int) channelStarter {
	return func(closeCh chan struct{}) *priorityChannel {
		pc := initPriorityChannel(closeCh)
		for i := 0; i < num; i++ {
			go func() {
				for {
					select {
					case <-closeCh:
						return
					case iReq := <-pc.Out:
						req := iReq.(requestPayload)
						res := &httpResponseResult{}
						res.res, res.err = req.responder()
						select {
						case <-closeCh:
							return
						case req.resCh <- res:
						}
					}
				}
			}()
		}
		return pc
	}
}
