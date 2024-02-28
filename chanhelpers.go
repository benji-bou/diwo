package chantools

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type newChanOptions[I any] func(ct *chanToolsConfig[I])
type ForwardToExtensions[I any] func(c I) error

// var ErrorLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgRed))
// var WarningLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgYellow))
// var InfoLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgBlue))

type chanToolsConfig[T any] struct {
	output          chan T
	errC            chan error
	b               int64
	initialValue    T
	hasInitialValue bool
	lastValue       T
	hasLastValue    bool
	params          []any
}

func WithInitialValue[I any](initialValue I) func(ct *chanToolsConfig[I]) {
	return func(ct *chanToolsConfig[I]) {
		ct.initialValue = initialValue
		ct.hasInitialValue = true
	}
}
func WithLastValue[I any](lasValue I) func(ct *chanToolsConfig[I]) {
	return func(ct *chanToolsConfig[I]) {
		ct.lastValue = lasValue
		ct.hasLastValue = true
	}
}

func withErrChan[I any]() func(ct *chanToolsConfig[I]) {
	return func(ct *chanToolsConfig[I]) {
		ct.errC = make(chan error)
	}
}

func WithBuffer[I any](count int64) func(ct *chanToolsConfig[I]) {
	return func(ct *chanToolsConfig[I]) {
		ct.b = count
	}
}
func WithParam[I any](p ...any) func(ct *chanToolsConfig[I]) {
	return func(ct *chanToolsConfig[I]) {
		ct.params = append(ct.params, p...)
	}
}

func Merge[E any](inputs ...<-chan E) <-chan E {
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

func new[C any](option ...newChanOptions[C]) chanToolsConfig[C] {
	cc := chanToolsConfig[C]{
		output:          make(chan C),
		errC:            nil,
		hasInitialValue: false,
		hasLastValue:    false,
		b:               0,
		params:          []any{},
	}
	for _, o := range option {
		o(&cc)
	}
	return cc
}

func Once[C any](value C) <-chan C {
	return New(func(c chan<- C, params ...any) {}, WithLastValue[C](value))
}

func New[C any](worker func(c chan<- C, params ...any), option ...newChanOptions[C]) <-chan C {
	cc := new(option...)
	go func(cc chanToolsConfig[C]) {
		defer close(cc.output)
		if cc.hasInitialValue {
			cc.output <- cc.initialValue
		}
		worker(cc.output, cc.params...)
		if cc.hasLastValue {
			cc.output <- cc.lastValue
		}
	}(cc)
	return cc.output
}

func NewWithErr[C any](worker func(c chan<- C, eC chan<- error, params ...any), option ...newChanOptions[C]) (<-chan C, <-chan error) {
	option = append(option, withErrChan[C]())
	cc := new(option...)
	go func(cc chanToolsConfig[C]) {
		uuid := uuid.NewString()
		slog.Debug("starting new go routine")
		defer close(cc.output)
		defer close(cc.errC)
		if cc.hasInitialValue {
			cc.output <- cc.initialValue
		}
		worker(cc.output, cc.errC, cc.params...)
		if cc.hasLastValue {
			cc.output <- cc.lastValue
		}
		slog.Debug("stop go routine")
	}(cc)
	return cc.output, cc.errC
}

func MapChan[I any, O any](input <-chan I, mapper func(input I) (O, error), option ...newChanOptions[O]) (<-chan O, <-chan error) {
	return NewWithErr(func(c chan<- O, eC chan<- error, params ...any) {
		for inputData := range input {
			output, err := mapper(inputData)
			if err != nil {
				eC <- err
			} else {
				c <- output

			}
		}
	}, option...)
}

func FlatChan[I any](input <-chan []I) <-chan I {
	return New(func(c chan<- I, params ...any) {
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

func ForwardIf[T any](src <-chan T, where func(element T) bool, option ...newChanOptions[T]) <-chan T {
	return New[T](func(c chan<- T, params ...any) {
		for s := range src {
			if where(s) {
				c <- s
			}
		}
	}, option...)
}

func BroadcastSync[T any](src <-chan T, qty uint) []<-chan T {
	dst := MakeSliceChan[T](qty)
	go func(src <-chan T, dst ...chan T) {
		defer func(dst []chan T) {
			for _, t := range dst {
				close(t)
			}
		}(dst)
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
		defer func(dst []chan T) {
			for _, t := range dst {
				close(t)
			}
		}(dst)
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
	mi, _ := MapChan(ch, func(input T) (any, error) { return input, nil })
	return mi
}

func CastTo[T any](ch <-chan any) <-chan T {
	mi, _ := MapChan(ch, func(input any) (T, error) { return input.(T), nil })
	return mi
}

func MapSlice[I any, O any](input []I, mapper func(input I) O) []O {
	res := make([]O, len(input))
	for i := 0; i < len(input); i++ {
		res[i] = mapper(input[i])
	}
	return res
}

func ForEach[T any](src <-chan T, each func(element T)) {
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

// func Map[S any, D any](src []S, mapper func(element S) D) []D {
// 	res := make([]D, len(src))
// 	for i, e := range src {
// 		res[i] = mapper(e)
// 	}
// 	return res
// }

// func ForEach[T any](inputSlice []T, each func(input T)) {
// 	for _, input := range inputSlice {
// 		each(input)
// 	}
// }

// func MergeMap[K comparable, V any](elem ...map[K]V) (map[K]V, error) {
// 	if len(elem) == 0 {
// 		return nil, fmt.Errorf("Empty input")
// 	} else if len(elem) == 1 {
// 		return elem[0], nil
// 	} else {
// 		res := elem[0]
// 		for i := 1; i < len(elem); i++ {
// 			toMerge := elem[i]
// 			for k, v := range toMerge {
// 				if _, exist := res[k]; exist {
// 					log.Printf("key %v already exist", k)
// 					return nil, fmt.Errorf("key %v already exist", k)
// 				}
// 				res[k] = v
// 			}
// 		}
// 		return res, nil
// 	}

// }

func Tick[I any](ctx context.Context, interval time.Duration, generate func() (I, error)) (<-chan I, <-chan error) {
	tick := time.NewTicker(interval)
	return NewWithErr(func(c chan<- I, errC chan<- error, params ...any) {
		for {
			select {
			case <-tick.C:
				elem, err := generate()
				if err != nil {
					errC <- err
				} else {
					c <- elem
				}
			case <-ctx.Done():
				tick.Stop()
				return
			}
		}
	})
}
