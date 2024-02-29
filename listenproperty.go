package chantools

import (
	"context"
	"sync"
)

type ListenProperty[E any] struct {
	value  E
	c      chan<- E
	ctx    context.Context
	broker *BrokerChan[E]
}

func Listen[E any](initialProperty E) *ListenProperty[E] {
	return ListenWithContext(context.Background(), initialProperty)
}

func ListenChan[E any](ctx context.Context, initialProperty E, src <-chan E) *ListenProperty[E] {
	listenProp := ListenWithContext(ctx, initialProperty)
	listenProp.Forward(ctx, src)
	return listenProp
}

func ListenWithContext[E any](ctx context.Context, initialProperty E) *ListenProperty[E] {
	c := make(chan E)
	lp := &ListenProperty[E]{value: initialProperty, c: c, broker: NewBrokerChan(c), ctx: ctx}
	return lp
}

func (lp ListenProperty[E]) Value() E {
	return lp.value
}

func (lp *ListenProperty[E]) Set(value E) {
	lp.value = value
	go func(value E) {
		lp.c <- value
	}(value)
}

func (lp *ListenProperty[E]) Forward(ctx context.Context, i <-chan E) {
	ForEach(i, func(element E) {
		lp.Set(element)
	})
}

func (lp ListenProperty[E]) Updates() <-chan E {
	return lp.broker.Subscribe()
}

func (lp ListenProperty[E]) Close() {
	close(lp.c)
}

func NewBrokerChan[E any](src <-chan E) *BrokerChan[E] {

	b := &BrokerChan[E]{src: src, listeners: make([]chan E, 0), newListener: make(chan chan E)}
	b.run()
	return b
}

type BrokerChan[E any] struct {
	src         <-chan E
	listeners   []chan E
	newListener chan chan E
}

func (bc *BrokerChan[E]) run() {
	go func() {
		var wg sync.WaitGroup
		defer func() {
			wg.Wait()
			for _, l := range bc.listeners {
				close(l)

			}
			close(bc.newListener)
		}()
		for {
			select {
			case s, isOk := <-bc.src:
				if !isOk {
					return
				}
				for _, l := range bc.listeners {
					wg.Add(1)
					go func(v E, l chan E) {
						l <- v
						wg.Done()
					}(s, l)
				}
			case nl := <-bc.newListener:
				bc.listeners = append(bc.listeners, nl)
			}
		}
	}()
}

func (bc *BrokerChan[E]) Subscribe() <-chan E {
	l := make(chan E)
	bc.newListener <- l
	return l
}
