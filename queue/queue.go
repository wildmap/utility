package queue

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/segmentio/ksuid"
)

type Handle[V any] func(data V)

type IQueue[K comparable, V any] interface {
	Publish(key K, data V)
	Subscribe(ctx context.Context, key K, handle Handle[V])
	Unsubscribe(key K)
	Close()
}

type Queue[K comparable, V any] struct {
	directs sync.Map
}

func NewQueue[K comparable, V any]() *Queue[K, V] {
	return &Queue[K, V]{}
}

func (q *Queue[K, V]) Publish(key K, data V) {
	d, _ := q.directs.LoadOrStore(key, newDirect[K, V](key))
	d.(*direct[K, V]).Publish(data)
}

func (q *Queue[K, V]) Subscribe(ctx context.Context, key K, handler Handle[V]) {
	d, _ := q.directs.LoadOrStore(key, newDirect[K, V](key))
	d.(*direct[K, V]).Subscribe(ctx, handler)
}

func (q *Queue[K, V]) Unsubscribe(key K) {
	d, ok := q.directs.Load(key)
	if !ok {
		return
	}
	d.(*direct[K, V]).Close()
	q.directs.Delete(key)
}

func (q *Queue[K, V]) Close() {
	q.directs.Range(func(k, v any) bool {
		v.(*direct[K, V]).Close()
		return true
	})
}

// 泛型direct
type direct[K comparable, V any] struct {
	name    K
	ch      chan V
	subs    []*sub[V]
	unacked int32
	closed  atomic.Bool
	mu      sync.RWMutex
	wg      sync.WaitGroup
}

type sub[V any] struct {
	ctx    context.Context
	cancel context.CancelFunc
	cname  string
}

func newDirect[K comparable, V any](name K) *direct[K, V] {
	return &direct[K, V]{
		name: name,
		ch:   make(chan V, 1024),
		subs: make([]*sub[V], 0),
	}
}

func (d *direct[K, V]) Name() K {
	return d.name
}

func (d *direct[K, V]) Close() {
	if !d.closed.CompareAndSwap(false, true) {
		return
	}
	close(d.ch)

	d.mu.Lock()
	for _, _sub := range d.subs {
		_sub.cancel()
	}
	d.mu.Unlock()

	d.wg.Wait()
}

func (d *direct[K, V]) Publish(data V) {
	if d.closed.Load() {
		return
	}
	select {
	case d.ch <- data:
	default:
		// queue full, dropping message
	}
}

func (d *direct[K, V]) Subscribe(ctx context.Context, handle Handle[V]) {
	subCtx, cancel := context.WithCancel(ctx)
	_sub := &sub[V]{
		cname:  ksuid.New().String(),
		ctx:    subCtx,
		cancel: cancel,
	}

	d.mu.Lock()
	d.subs = append(d.subs, _sub)
	d.mu.Unlock()

	d.wg.Add(1)

	go func() {
		defer d.wg.Done()
		defer cancel()

		for {
			select {
			case <-_sub.ctx.Done():
				d.removeSubscriber(_sub.cname)
				return
			case msg, ok := <-d.ch:
				if !ok {
					return
				}
				atomic.AddInt32(&d.unacked, 1)
				if handle != nil {
					handle(msg)
				}
				atomic.AddInt32(&d.unacked, -1)
			}
		}
	}()
}

func (d *direct[K, V]) removeSubscriber(cname string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, _sub := range d.subs {
		if _sub.cname == cname {
			d.subs = append(d.subs[:i], d.subs[i+1:]...)
			break
		}
	}
}
