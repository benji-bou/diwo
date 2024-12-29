package diwo

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Options func(ct *produceConfig)
type ForwardToExtensions[I any] func(c I) error

// var ErrorLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgRed))
// var WarningLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgYellow))
// var InfoLog = pterm.DefaultBasicText.WithStyle(pterm.NewStyle(pterm.FgBlue))

type produceConfig struct {
	isNonBlocking  bool
	shouldNotClose bool
	params         []any
	name           string
	tearDown       func()
}

func newDefaultProduceConfig() produceConfig {
	return produceConfig{
		params:         []any{},
		isNonBlocking:  false,
		shouldNotClose: false,
		name:           uuid.NewString(),
	}
}

func WithTearDown(td func()) Options {
	return func(ct *produceConfig) {
		ct.tearDown = td
	}
}

func WithName(name string) Options {
	return func(ct *produceConfig) {
		ct.name = name
	}
}

func WithParam(p ...any) Options {
	return func(ct *produceConfig) {
		ct.params = append(ct.params, p...)
	}
}

func WithNonBlocking() Options {
	return func(ct *produceConfig) {
		ct.isNonBlocking = true
	}
}

func newChanConfig(option ...Options) produceConfig {
	cc := newDefaultProduceConfig()
	for _, o := range option {
		o(&cc)
	}
	return cc
}

type WorkerChanBuilder[C any] func() chan C

func defaultOutput[C any]() chan C {
	o := make(chan C)
	return o
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
func infiniteWrapper[C any](workerC chan<- C) chan C {
	inputC := make(chan C)
	go startInfinitBroker(inputC, workerC)
	return inputC
}

func start[C any](cc produceConfig, startWorker func(inputC chan<- C), workerCBuilder WorkerChanBuilder[C]) <-chan C {
	workerC := workerCBuilder()
	go func() {

		defer func() {
			if cc.tearDown != nil {
				cc.tearDown()
			}
		}()
		if !cc.shouldNotClose {
			defer close(workerC)

		}

		if cc.isNonBlocking {
			workerC = infiniteWrapper(workerC)
		}
		startWorker(workerC)
	}()
	return workerC
}

func New[C any](worker func(c chan<- C), option ...Options) <-chan C {
	cc := newChanConfig(option...)
	return start(cc, worker, defaultOutput[C])
}

func NewWithChanBuilder[C any](builderC WorkerChanBuilder[C], worker func(c chan<- C), option ...Options) <-chan C {
	cc := newChanConfig(option...)
	return start(cc, worker, builderC)
}

func Once[C any](value C) <-chan C {
	return New(func(c chan<- C) { c <- value })
}

func FromSlice[C any, A ~[]C](input A) <-chan C {
	return New(func(c chan<- C) {
		for _, i := range input {
			c <- i
		}
	})
}

func Broadcast[T any](src <-chan T, qty uint) []<-chan T {
	dst := MakeSliceChan[T](qty)
	go func(src <-chan T, dst ...chan T) {
		wg := &sync.WaitGroup{}
		defer func(dst []chan T) {
			for _, t := range dst {
				close(t)
			}
		}(dst)
		for inputData := range src {
			for _, d := range dst {
				wg.Add(1)
				go func(inputData T, d chan<- T) {
					defer wg.Done()
					d <- inputData
				}(inputData, d)
			}
		}
		wg.Wait()
	}(src, dst...)
	return CastToReader(dst...)
}

func MakeSliceChan[T any](qty uint) []chan T {
	res := make([]chan T, qty)
	for i := range res {
		res[i] = make(chan T)
	}
	return res
}

func Tick[I any](ctx context.Context, interval time.Duration, generate func(t time.Time) I) <-chan I {
	tick := time.NewTicker(interval)
	return New(func(c chan<- I) {
		for {
			select {
			case t := <-tick.C:
				c <- generate(t)
			case <-ctx.Done():
				tick.Stop()
				return
			}
		}
	})
}
