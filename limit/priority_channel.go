package limit

// priorityChannel is the channels support the priority of them.
type priorityChannel struct {
	Out    chan interface{}
	High   chan interface{}
	Normal chan interface{}
	Low    chan interface{}
	stopCh chan struct{}
}

// initPriorityChannel will create the priorityChannel and initialize it
func initPriorityChannel(closeCh <-chan struct{}) *priorityChannel {
	pc := priorityChannel{}
	pc.Out = make(chan interface{})
	pc.High = make(chan interface{})
	pc.Normal = make(chan interface{})
	pc.Low = make(chan interface{})
	pc.stopCh = make(chan struct{})

	pc.start()
	return &pc
}

// Close will close all channels in it.
func (pc *priorityChannel) Close() {
	close(pc.stopCh)
	close(pc.High)
	close(pc.Normal)
	close(pc.Low)
	close(pc.Out)
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
