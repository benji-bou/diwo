package chantools

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type NewChanOptions[I any] func(ct *chanToolsConfig[I])
type ForwardToExtensions[I any] func(c I) error

// var ErrorLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgRed))
// var WarningLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgYellow))
// var InfoLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgBlue))

type chanToolsConfig[T any] struct {
	outputC         chan T
	errC            chan error
	initialValue    T
	hasInitialValue bool
	lastValue       T
	hasLastValue    bool
	params          []any
	isNonBlocking   bool
	shouldNotClose  bool
	name            string
	ctx             context.Context
}

func WithName[I any](name string) NewChanOptions[I] {
	return func(ct *chanToolsConfig[I]) {
		ct.name = name
	}
}
func WithNonManagedChannel[I any]() NewChanOptions[I] {
	return func(ct *chanToolsConfig[I]) {
		ct.shouldNotClose = true
	}
}

func WithInitialValue[I any](initialValue I) NewChanOptions[I] {
	return func(ct *chanToolsConfig[I]) {
		ct.initialValue = initialValue
		ct.hasInitialValue = true
	}
}
func WithLastValue[I any](lasValue I) NewChanOptions[I] {
	return func(ct *chanToolsConfig[I]) {
		ct.lastValue = lasValue
		ct.hasLastValue = true
	}
}

func withErrChan[I any]() NewChanOptions[I] {
	return NewChanOptions[I](func(ct *chanToolsConfig[I]) {
		ct.errC = make(chan error)
	})
}

func WithBuffer[I any](count int64) NewChanOptions[I] {
	return func(ct *chanToolsConfig[I]) {
		ct.outputC = make(chan I, count)
	}
}
func WithParam[I any](p ...any) NewChanOptions[I] {
	return func(ct *chanToolsConfig[I]) {
		ct.params = append(ct.params, p...)
	}
}

func WithNonBlocking[I any]() NewChanOptions[I] {
	return func(ct *chanToolsConfig[I]) {
		ct.isNonBlocking = true
	}
}

// WithCOntext option will prevent the go routine to finish until the context.Done() chan return a result
func WithContext[I any](ctx context.Context) NewChanOptions[I] {
	return func(ct *chanToolsConfig[I]) {
		ct.ctx = ctx
	}
}

func Merge[E any, C <-chan E](inputs ...C) C {
	var wg sync.WaitGroup
	merged := make(chan E, 10000)
	wg.Add(len(inputs))
	outputC := func(sc <-chan E) {
		for sqr := range sc {
			merged <- sqr
		}
		wg.Done()
	}
	for _, optChan := range inputs {
		if optChan == nil {
			wg.Done()
		} else {
			go outputC(optChan)
		}
	}
	go func() {
		wg.Wait()
		close(merged)
	}()
	return merged
}

func Concat[E any, C <-chan E](inputs ...C) []E {
	outputC := Merge(inputs...)
	res := make([]E, 0, len(inputs))
	for elem := range outputC {
		res = append(res, elem)
	}
	return res
}

func newChanConfig[C any](option ...NewChanOptions[C]) chanToolsConfig[C] {
	cc := chanToolsConfig[C]{
		outputC:         nil,
		errC:            nil,
		hasInitialValue: false,
		hasLastValue:    false,
		params:          []any{},
		isNonBlocking:   false,
		shouldNotClose:  false,
		name:            uuid.NewString(),
	}
	for _, o := range option {
		o(&cc)
	}
	if cc.outputC == nil {
		cc.outputC = make(chan C)
	}
	return cc
}

func (cc chanToolsConfig[C]) start(startWorker func(inputC chan<- C, inputErrC chan<- error, params ...any)) {
	if cc.isNonBlocking {
		go cc.infiniteImpl(startWorker)
	} else {
		go cc.defaultImpl(startWorker)
	}

}

// startInfinitBroker transfer inputC data to outputC. the caller can write to inputC without blocking.
// this function takes outputC ownership. Which means it handle closing outputC when no more data is available and inputC is closed
func startInfinitBroker[T any](inputC <-chan T, outputC chan<- T) {
	go func() {
		defer close(outputC)
		buff := make([]T, 0)
		for {
			var nextData T
			var nextUpdateC chan<- T
			if len(buff) > 0 {
				nextData = buff[0]
				nextUpdateC = outputC
			} else if inputC == nil {
				return
			}
			select {
			case inputData, ok := <-inputC:
				if !ok {
					inputC = nil
					continue
				}
				buff = append(buff, inputData)

			case nextUpdateC <- nextData:
				buff = buff[1:]

			}
		}
	}()
}

// solution from https://go.dev/talks/2013/advconc.slide#30
func (cc chanToolsConfig[C]) infiniteImpl(startWorker func(inputC chan<- C, inputErrC chan<- error, params ...any)) {
	inputC := make(chan C)
	if !cc.shouldNotClose {
		defer func() { slog.Debug("close inputC from infiniteImple", "inputC", inputC); close(inputC) }()

	}
	defer func() {
		if cc.errC != nil {
			close(cc.errC)
		}
	}()

	go startInfinitBroker(inputC, cc.outputC)

	slog.Debug("starting new go routine", "id", cc.name)

	if cc.hasInitialValue {
		inputC <- cc.initialValue
	}
	startWorker(inputC, cc.errC, cc.params...)
	if cc.hasLastValue {
		inputC <- cc.lastValue
	}
	if cc.ctx != nil {
		<-cc.ctx.Done()
	}
	slog.Debug("stop go routine", "id", cc.name)

}

func (cc chanToolsConfig[C]) defaultImpl(startWorker func(inputC chan<- C, inputErrC chan<- error, params ...any)) {

	slog.Debug("starting new go routine", "id", cc.name)
	if !cc.shouldNotClose {
		defer close(cc.outputC)

	}
	defer func() {
		if cc.errC != nil {
			close(cc.errC)
		}
	}()
	if cc.hasInitialValue {
		cc.outputC <- cc.initialValue
	}
	startWorker(cc.outputC, cc.errC, cc.params...)
	if cc.hasLastValue {
		cc.outputC <- cc.lastValue
	}
	if cc.ctx != nil {
		<-cc.ctx.Done()
	}
	slog.Debug("stop go routine", "id", cc.name)
}

func Once[C any](value C) <-chan C {
	return New(func(c chan<- C, params ...any) {}, WithLastValue[C](value))
}

func New[C any](worker func(c chan<- C, params ...any), option ...NewChanOptions[C]) <-chan C {
	cc := newChanConfig(option...)
	cc.start(func(inputC chan<- C, inputErrC chan<- error, params ...any) { worker(inputC, cc.params...) })
	return cc.outputC
}

func NewWithErr[C any](worker func(c chan<- C, eC chan<- error, params ...any), option ...NewChanOptions[C]) (<-chan C, <-chan error) {
	option = append(option, withErrChan[C]())
	cc := newChanConfig(option...)
	cc.start(func(inputC chan<- C, inputErrC chan<- error, params ...any) { worker(inputC, inputErrC, params...) })
	return cc.outputC, cc.errC
}

func NewSync[C any](worker func(c chan<- C, params ...any), option ...NewChanOptions[C]) []C {
	cc := newChanConfig(option...)
	cc.start(func(inputC chan<- C, inputErrC chan<- error, params ...any) { worker(inputC, cc.params...) })
	return Concat(CastToReader(cc.outputC)...)
}

func NewSyncWithErr[C any](worker func(c chan<- C, eC chan<- error, params ...any), option ...NewChanOptions[C]) ([]C, []error) {
	option = append(option, withErrChan[C]())
	cc := newChanConfig(option...)
	cc.start(func(inputC chan<- C, inputErrC chan<- error, params ...any) { worker(inputC, inputErrC, params...) })
	return Concat(CastToReader(cc.outputC)...), Concat(CastToReader(cc.errC)...)
}

func Map[I any, O any](input <-chan I, mapper func(input I) O, option ...NewChanOptions[O]) <-chan O {
	return New(func(c chan<- O, params ...any) {
		for inputData := range input {
			outputC := mapper(inputData)
			c <- outputC
		}
	}, option...)
}

func Flatten[I any](input <-chan []I) <-chan I {
	return New(func(c chan<- I, params ...any) {
		for i := range input {
			for _, v := range i {
				c <- v
			}
		}
	})
}

func ForwardTo[T any](ctx context.Context, src <-chan T, dst chan<- T) {
	go func() {
		for {
			select {
			case value, ok := <-src:
				if !ok {
					return
				}
				dst <- value
			case <-ctx.Done():
				return
			}
		}
	}()

}

func ForwardIf[T any](src <-chan T, where func(element T) bool, option ...NewChanOptions[T]) <-chan T {
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
	return CastToReader(dst...)
}

func Broadcast[T any](src <-chan T, qty uint) []<-chan T {
	dst := MakeSliceChan[T](qty)
	done := make(chan struct{}, 0)
	go func(src <-chan T, dst ...chan T) {
		defer func(dst []chan T) {
			close(done)
			for _, t := range dst {
				close(t)
			}
		}(dst)
		for inputData := range src {
			for _, d := range dst {
				go func(inputData T, d chan<- T) {
					select {
					case d <- inputData:
					case <-done:

					}
				}(inputData, d)
			}
		}
	}(src, dst...)
	return CastToReader(dst...)
}

func CastToReader[T any](ch ...chan T) []<-chan T {
	return MapSlice(ch, func(input chan T) <-chan T {
		return input
	})
}

func CastToAny[T any](ch <-chan T) <-chan any {
	return Map(ch, func(input T) any { return input })

}

func CastTo[T any](ch <-chan any) <-chan T {
	return Map(ch, func(input any) T { return input.(T) })
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
