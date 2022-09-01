package helpers

import (
	"context"
	"log"
	"sync"
)

type ChanGeneratorExtension[I any] func(c chan<- I)
type ForwardToExtensions[I any] func(c I) error

// var ErrorLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgRed))
// var WarningLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgYellow))
// var InfoLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgBlue))

func WithInitialValue[I any](initialValue I) func(c chan<- I) {
	return func(c chan<- I) {
		c <- initialValue
	}
}

func MergeChan[E any](inputs ...<-chan E) <-chan E {
	var wg sync.WaitGroup
	merged := make(chan E, 10000)
	wg.Add(len(inputs))
	output := func(sc <-chan E) {
		for sqr := range sc {
			merged <- sqr
		}
		wg.Done()
	}
	for _, optChan := range inputs {
		if optChan == nil {
			wg.Done()
		} else {
			go output(optChan)
		}
	}
	go func() {
		wg.Wait()
		close(merged)
	}()
	return merged
}

func ChanBuffGenerator[C any](worker func(chan<- C), count int, extensions ...ChanGeneratorExtension[C]) <-chan C {
	output := make(chan C, count)
	go func(chan<- C) {
		defer close(output)
		if len(extensions) > 0 {
			extensions[0](output)
		}
		worker(output)
		if len(extensions) > 1 {
			extensions[1](output)
		}
	}(output)
	return output
}

func ChanBuffErrGenerator[C any](worker func(chan<- C, chan<- error), count int, extensions ...ChanGeneratorExtension[C]) (<-chan C, <-chan error) {
	output := make(chan C, count)
	err := make(chan error)
	go func(o chan<- C, e chan<- error) {
		defer close(o)
		defer close(err)
		if len(extensions) > 0 {
			extensions[0](o)
		}
		worker(o, e)
		if len(extensions) > 1 {
			extensions[1](o)
		}
	}(output, err)
	return output, err
}

func ChanGenerator[C any](worker func(chan<- C), extensions ...ChanGeneratorExtension[C]) <-chan C {
	output := make(chan C)
	go func(chan<- C) {
		defer close(output)
		if len(extensions) > 0 {
			extensions[0](output)
		}
		worker(output)
		if len(extensions) > 1 {
			extensions[1](output)
		}
	}(output)
	return output
}

func ChanErrGenerator[C any](worker func(chan<- C, chan<- error), extensions ...ChanGeneratorExtension[C]) (<-chan C, <-chan error) {
	output := make(chan C)
	err := make(chan error)
	go func(o chan<- C, e chan<- error) {
		defer close(o)
		if len(extensions) > 0 {
			extensions[0](o)
		}
		worker(o, e)
		if len(extensions) > 1 {
			extensions[1](o)
		}
	}(output, err)
	return output, err
}

func MapChan[I any, O any](input <-chan I, mapper func(input I) (O, error), extensions ...ChanGeneratorExtension[O]) <-chan O {
	return ChanGenerator(func(c chan<- O) {
		for inputData := range input {
			output, err := mapper(inputData)
			if err != nil {
				log.Printf("Mapping failed with err: %v", err)
				return
			}
			c <- output
		}
	}, extensions...)
}

func FlatChan[I any](input <-chan []I) <-chan I {
	return ChanGenerator(func(c chan<- I) {
		for i := range input {
			for _, v := range i {
				c <- v
			}
		}
	})
}

func ForwardTo[T any](ctx context.Context, src <-chan T, dst chan<- T, extension ...ForwardToExtensions[T]) {
	go func() {
		for {
			select {
			case value, ok := <-src:
				if !ok {
					return
				}
				for _, e := range extension {
					e(value)
				}
				dst <- value
			case <-ctx.Done():
				return
			}
		}
	}()

}

func ForwardIf[T any](src <-chan T, where func(element T) bool, extensions ...ChanGeneratorExtension[T]) <-chan T {
	return ChanGenerator[T](func(c chan<- T) {
		for s := range src {
			if where(s) {
				c <- s
			}
		}
	}, extensions...)
}

func BroadcastSync[T any](src <-chan T, qty uint) []<-chan T {
	dst := MakeSliceChan[T](qty)
	go func(src <-chan T, dst ...chan T) {
		defer ForEach(dst, func(input chan T) { close(input) })
		for inputData := range src {
			for _, d := range dst {
				d <- inputData
			}
		}
	}(src, dst...)
	return CastToReader(dst)
}

func Broadcast[T any](src <-chan T, qty uint) []<-chan T {
	dst := MakeSliceChan[T](qty)
	go func(src <-chan T, dst ...chan T) {
		defer ForEach(dst, func(input chan T) { close(input) })
		for inputData := range src {
			for _, d := range dst {
				go func(inputData T, d chan<- T) {
					d <- inputData
				}(inputData, d)
			}
		}
	}(src, dst...)
	return CastToReader(dst)
}

func CastToReader[T any](ch []chan T) []<-chan T {
	return MapSlice(ch, func(input chan T) <-chan T {
		return input
	})
}

func CastToAny[T any](ch <-chan T) <-chan any {
	return MapChan(ch, func(input T) (any, error) { return input, nil })
}

func CastTo[T any](ch <-chan any) <-chan T {
	return MapChan(ch, func(input any) (T, error) { return input.(T), nil })
}

func MapSlice[I any, O any](input []I, mapper func(input I) O) []O {
	res := make([]O, len(input))
	for i := 0; i < len(input); i++ {
		res[i] = mapper(input[i])
	}
	return res
}

func ForEachChan[T any](src <-chan T, each func(element T)) {
	go func() {
		for s := range src {
			each(s)
		}
	}()
}

func MakeSliceChan[T any](qty uint) []chan T {
	res := make([]chan T, qty)
	for i := range res {
		res[i] = make(chan T)
	}
	return res
}

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
	ForEachChan(i, func(element E) {
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

		defer func() {
			for _, l := range bc.listeners {
				close(l)
			}
		}()
		for {
			select {
			case s, isOk := <-bc.src:
				if !isOk {
					return
				}
				for _, l := range bc.listeners {
					go func(v E, l chan E) {
						l <- v
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

// func Map[S any, D any](src []S, mapper func(element S) D) []D {
// 	res := make([]D, len(src))
// 	for i, e := range src {
// 		res[i] = mapper(e)
// 	}
// 	return res
// }

func ForEach[T any](inputSlice []T, each func(input T)) {
	for _, input := range inputSlice {
		each(input)
	}
}
