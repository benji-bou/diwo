package diwo

import "sync"

func syncForward[E any](src <-chan E, dst chan<- E, wg *sync.WaitGroup) {
	defer wg.Done()
	for elem := range src {
		dst <- elem
	}
}

func Merge[E any, C <-chan E](inputs ...C) C {
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
				go syncForward(optChan, merged, wg)
			}
		}
		wg.Wait()
	})

}

func Concat[E any, C <-chan E](inputs ...C) []E {
	outputC := Merge(inputs...)
	res := make([]E, 0, len(inputs))
	for elem := range outputC {
		res = append(res, elem)
	}
	return res
}

func Map[I any, O any](input <-chan I, mapper func(input I) O, option ...Options) <-chan O {
	return New(func(c chan<- O) {
		for inputData := range input {
			outputC := mapper(inputData)
			c <- outputC
		}
	}, option...)
}

func Flatten[I any](input <-chan []I, option ...Options) <-chan I {
	return New(func(c chan<- I) {
		for i := range input {
			for _, v := range i {
				c <- v
			}
		}
	}, option...)
}

func ForwardTo[T any](src <-chan T, dst chan<- T) {
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
