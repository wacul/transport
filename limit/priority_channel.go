package limit

import (
	"sync"
)

// priorityChannel is the channels support the priority of them.
type priorityChannel struct {
	Out    chan interface{}
	High   chan interface{}
	Normal chan interface{}
	Low    chan interface{}
	stopCh chan struct{}

	refCount uint
	mu       *sync.Mutex
}

// initPriorityChannel will create the priorityChannel and initialize it
func initPriorityChannel(closeCh <-chan struct{}) *priorityChannel {
	pc := priorityChannel{}
	pc.Out = make(chan interface{})
	pc.High = make(chan interface{})
	pc.Normal = make(chan interface{})
	pc.Low = make(chan interface{})
	pc.stopCh = make(chan struct{})
	pc.mu = new(sync.Mutex)

	pc.start()
	return &pc
}

// Close will close all channels in it.
func (pc *priorityChannel) Close() {
	close(pc.stopCh)
}

func (pc *priorityChannel) add() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.refCount++
}

func (pc *priorityChannel) done() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.refCount--
}

func (pc *priorityChannel) using() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.refCount != 0
}

func (pc *priorityChannel) start() {
	go func() {
		for {
			select {
			case s := <-pc.High:
				pc.Out <- s
			case <-pc.stopCh:
				return
			default:
			}

			select {
			case s := <-pc.High:
				pc.Out <- s
			case s := <-pc.Normal:
				pc.Out <- s
			case <-pc.stopCh:
				return
			default:
			}

			select {
			case s := <-pc.High:
				pc.Out <- s
			case s := <-pc.Normal:
				pc.Out <- s
			case s := <-pc.Low:
				pc.Out <- s
			case <-pc.stopCh:
				return
			}
		}
	}()
}
