package diwo

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type (
	Options                    func(ct *produceConfig)
	ForwardToExtensions[I any] func(c I) error
)

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

func WithNonBlocking() Options {
	return func(ct *produceConfig) {
		ct.isNonBlocking = true
	}
}

func WithUnmanaged() Options {
	return func(ct *produceConfig) {
		ct.shouldNotClose = true
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
// this function takes outputC ownership. Which means it handles closing outputC when no more data is available and inputC is closed
func startInfinitBroker[T any](inputC <-chan T, outputC chan<- T) {
	defer close(outputC)
	inputCIsClosed := false
	buff := make([]T, 0)
	for {
		var nextData T
		// Define a nil chan
		var nextUpdateC chan<- T
		if len(buff) > 0 {
			nextData = buff[0]
			// if something to send nextUpdateC is no more nil
			nextUpdateC = outputC
		} else if inputCIsClosed {
			return
		}
		if !inputCIsClosed {
			select {
			case inputData, ok := <-inputC:
				if !ok {
					inputCIsClosed = true
					continue
				}
				buff = append(buff, inputData)
				// if nextUpdateC not nil can send data otherwise case not evaluated
			case nextUpdateC <- nextData:
				buff = buff[1:]

			}
		} else {
			nextUpdateC <- nextData
			buff = buff[1:]

		}
	}
}

// solution from https://go.dev/talks/2013/advconc.slide#30
func infiniteWrapper[C any](workerC chan<- C) chan<- C {
	inputC := make(chan C)
	go startInfinitBroker(inputC, workerC)
	return inputC
}

func start[C any](cc produceConfig, startWorker func(inputC chan<- C), outputC chan C, isInternalBuilder bool) <-chan C {
	go func() {
		defer func() {
			if cc.tearDown != nil {
				cc.tearDown()
			}
		}()
		if !cc.shouldNotClose || isInternalBuilder {
			defer close(outputC)
		}
		var inputC chan<- C = outputC
		if cc.isNonBlocking {
			inputC = infiniteWrapper(outputC)
		}
		startWorker(inputC)
	}()
	return outputC
}

func New[C any](worker func(c chan<- C), option ...Options) <-chan C {
	cc := newChanConfig(option...)
	if cc.shouldNotClose {
		slog.Warn("You declare an unmanged chan that you can't close yourself. So we will close it anyway. Please consider using NewWithChanBuilder instead")
	}
	return start(cc, worker, defaultOutput[C](), true)
}

func NewWithChan[C any](outputC chan C, worker func(c chan<- C), option ...Options) <-chan C {
	cc := newChanConfig(option...)
	return start(cc, worker, outputC, false)
}

func Once[C any](value C) <-chan C {
	return New(func(c chan<- C) { c <- value })
}

func Empty[C any]() <-chan C {
	emptyCh := make(chan C)
	close(emptyCh)
	return emptyCh
}

func FromSlice[C any, A ~[]C](input A) <-chan C {
	return New(func(c chan<- C) {
		for _, i := range input {
			c <- i
		}
	})
}

func FromSliceStarter[C any, A ~[]C](input A) (func(), <-chan C) {
	startC := make(chan struct{})
	return func() {
			close(startC)
		}, New(func(c chan<- C) {
			<-startC
			for _, i := range input {
				c <- i
			}
		})
}

func Broadcast[T any](src <-chan T, qty int) []<-chan T {
	switch qty {
	case 0:
		return []<-chan T{}
	case 1:
		return []<-chan T{src}
	default:
		dstFinals := MakeSliceChan[T](qty)
		go func() {
			dstInfinitWrappers := make([]chan<- T, 0, qty)
			for _, dst := range dstFinals {
				dstInfinitWrappers = append(dstInfinitWrappers, infiniteWrapper(dst))
			}
			for value := range src {
				for _, dst := range dstInfinitWrappers {
					dst <- value
				}
			}
			for _, dst := range dstInfinitWrappers {
				close(dst)
			}
		}()
		return CastToReader(dstFinals...)
	}
}

func MakeSliceChan[T any](qty int) []chan T {
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
