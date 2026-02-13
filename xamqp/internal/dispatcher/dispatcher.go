package dispatcher

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Dispatcher -
type Dispatcher struct {
	subscribers         map[int]dispatchSubscriber
	subscribersMu       *sync.Mutex
	subscriberIDCounter atomic.Int64
}

type dispatchSubscriber struct {
	notifyCancelOrCloseChan chan error
	closeCh                 <-chan struct{}
}

// New -
func New() *Dispatcher {
	return &Dispatcher{
		subscribers:   make(map[int]dispatchSubscriber),
		subscribersMu: &sync.Mutex{},
	}
}

// Dispatch -
func (d *Dispatcher) Dispatch(err error) error {
	d.subscribersMu.Lock()
	defer d.subscribersMu.Unlock()
	for _, subscriber := range d.subscribers {
		select {
		case <-time.After(time.Second * 5):
			slog.Warn("Unexpected rabbitmq error: timeout in dispatch")
		case subscriber.notifyCancelOrCloseChan <- err:
		}
	}
	return nil
}

// AddSubscriber -
func (d *Dispatcher) AddSubscriber() (<-chan error, chan<- struct{}) {
	id := int(d.subscriberIDCounter.Add(1))

	closeCh := make(chan struct{})
	notifyCancelOrCloseChan := make(chan error)

	d.subscribersMu.Lock()
	d.subscribers[id] = dispatchSubscriber{
		notifyCancelOrCloseChan: notifyCancelOrCloseChan,
		closeCh:                 closeCh,
	}
	d.subscribersMu.Unlock()

	go func(id int) {
		<-closeCh
		d.subscribersMu.Lock()
		defer d.subscribersMu.Unlock()
		sub, ok := d.subscribers[id]
		if !ok {
			return
		}
		close(sub.notifyCancelOrCloseChan)
		delete(d.subscribers, id)
	}(id)
	return notifyCancelOrCloseChan, closeCh
}
