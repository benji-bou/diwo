package diwo

import (
	"sync"
)

func NewBroker[E any](src <-chan E) *Broker[E] {
	if src == nil {
		src = Empty[E]()
	}
	b := &Broker[E]{
		src:             src,
		internalBridges: make(map[chan<- E]struct{}, 0),
		mux:             &sync.RWMutex{},
		isSrcClosed:     false,
	}

	go b.run()
	return b
}

type Broker[E any] struct {
	src             <-chan E
	internalBridges map[chan<- E]struct{}
	isSrcClosed     bool

	mux *sync.RWMutex
}

func (bc *Broker[E]) cleanup() {
	for l := range bc.internalBridges {
		close(l)
		delete(bc.internalBridges, l)
	}
}

func (bc *Broker[E]) startNewListener(listener chan E) {
	bridge := infiniteWrapper(listener)
	bc.internalBridges[bridge] = struct{}{}
}

func (bc *Broker[E]) run() {
	defer func() {
		bc.mux.Lock()
		bc.isSrcClosed = true
		bc.cleanup()
		bc.mux.Unlock()
	}()

	for s := range bc.src {
		bc.mux.RLock()
		for l := range bc.internalBridges {
			l <- s
		}
		bc.mux.RUnlock()
	}
}

func (bc *Broker[E]) Subscribe() <-chan E {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if bc.isSrcClosed {
		return Empty[E]()
	}
	l := make(chan E)
	bc.startNewListener(l)
	return l

}
