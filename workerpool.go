package diwo

func Pool[R any](qty int, worker func(outputC chan<- R)) []<-chan R {
	resC := make([]<-chan R, 0, qty)
	for range qty {
		resWrokerC := New(worker)
		resC = append(resC, resWrokerC)
	}
	return resC
}

func MergedPool[S any, R any](qty int, worker func(outputC chan<- R)) <-chan R {
	return Merge(Pool(qty, worker)...)
}

func PoolDispatch[S any, R any](qty int, src <-chan S, worker func(inputValue S, outputC chan<- R)) []<-chan R {
	resC := make([]<-chan R, 0, qty)
	for range qty {
		resWrokerC := New(func(c chan<- R) {
			for nextValue := range src {
				worker(nextValue, c)
			}
		})
		resC = append(resC, resWrokerC)
	}
	return resC
}

func MergedPoolDispatch[S any, R any](qty int, src <-chan S, worker func(inputValue S, outputC chan<- R)) <-chan R {
	return Merge(PoolDispatch(qty, src, worker)...)
}

func SlicedPool[S any, SA ~[]S, R any](src SA, worker func(inputValue S, outputC chan<- R)) []<-chan R {
	qty := len(src)
	resC := make([]<-chan R, 0, qty)
	for _, value := range src {
		resWrokerC := New(func(c chan<- R) {
			worker(value, c)
		})
		resC = append(resC, resWrokerC)
	}
	return resC
}

func MergedSlicedPool[S any, SA ~[]S, R any](src SA, worker func(inputValue S, outputC chan<- R)) <-chan R {
	return Merge(SlicedPool(src, worker)...)
}
