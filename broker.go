package diwo

import (
	"sync"
)

func NewBroker[E any](src <-chan E) *Broker[E] {

	b := &Broker[E]{src: src, listeners: make([]chan E, 0), newListener: make(chan chan E)}
	b.run()
	return b
}

type Broker[E any] struct {
	src         <-chan E
	listeners   []chan E
	newListener chan chan E
}

func (bc *Broker[E]) run() {
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

func (bc *Broker[E]) Subscribe() <-chan E {
	l := make(chan E)
	bc.newListener <- l
	return l
}
