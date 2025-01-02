package diwo

import (
	"slices"
	"sync"
)

func syncForward[E any](src <-chan E, dst chan<- E) {
	for elem := range src {
		dst <- elem
	}
}

type UnionOperatorFunc[E any] func(inputs ...<-chan E) <-chan E

func Merge[E any](inputs ...<-chan E) <-chan E {
	if inputs == nil {
		return Empty[E]()
	}
	inputs = slices.DeleteFunc(inputs, func(elem <-chan E) bool { return elem == nil })
	if len(inputs) == 0 {
		return Empty[E]()
	}
	if len(inputs) == 1 {
		return inputs[0]
	}
	return New(func(merged chan<- E) {
		wg := &sync.WaitGroup{}
		wg.Add(len(inputs))
		for _, optChan := range inputs {
			if optChan == nil {
				wg.Done()
			} else {
				go func() {
					defer wg.Done()
					syncForward(optChan, merged)
				}()
			}
		}
		wg.Wait()
	}, WithUnmanaged())
}

func Concat[E any](inputs ...<-chan E) <-chan E {
	if inputs == nil {
		return Empty[E]()
	}
	inputs = slices.DeleteFunc(inputs, func(elem <-chan E) bool { return elem == nil })
	if len(inputs) == 0 {
		return Empty[E]()
	}
	if len(inputs) == 1 {
		return inputs[0]
	}
	return New(func(c chan<- E) {
		for _, input := range inputs {
			for value := range input {
				c <- value
			}
		}
	})
}

type CollectorFunc[E any] func(inputs ...<-chan E) []E

func Collect[E any](strat UnionOperatorFunc[E]) CollectorFunc[E] {
	return func(inputs ...<-chan E) []E {
		outputC := strat(inputs...)
		res := make([]E, 0, len(inputs))
		for elem := range outputC {
			res = append(res, elem)
		}
		return res
	}
}

func CollectMerge[E any](inputs ...<-chan E) []E {
	return Collect[E](Merge)(inputs...)
}

func CollectConcat[E any](inputs ...<-chan E) []E {
	return Collect[E](Concat)(inputs...)
}

func Map[I any, O any](input <-chan I, mapper func(input I) O, option ...Options) <-chan O {
	return New(func(c chan<- O) {
		for inputData := range input {
			outputC := mapper(inputData)
			c <- outputC
		}
	}, option...)
}

func ForwardTo[T any](src <-chan T, dst chan<- T) {
	defer close(dst)
	for elem := range src {
		dst <- elem
	}
}

func Filter[T any](src <-chan T, where func(element T) bool, option ...Options) <-chan T {
	return New[T](func(c chan<- T) {
		for s := range src {
			if where(s) {
				c <- s
			}
		}
	}, option...)
}

func mapSlice[I any, IA ~[]I, O any](input IA, mapper func(input I) O) []O {
	res := make([]O, len(input))
	for i := 0; i < len(input); i++ {
		res[i] = mapper(input[i])
	}
	return res
}

func CastToReader[T any](ch ...chan T) []<-chan T {
	return mapSlice(ch, func(input chan T) <-chan T {
		return input
	})
}

func CastToAny[T any](ch <-chan T) <-chan any {
	return Map(ch, func(input T) any { return input })

}

func CastTo[T any](ch <-chan any) <-chan T {
	return Map(ch, func(input any) T { return input.(T) })
}
