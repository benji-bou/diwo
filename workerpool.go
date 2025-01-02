package diwo

func NewPool[S any, R any](qty int, src <-chan S, worker func(inputValue S, outputC chan<- R)) []<-chan R {
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

func NewMergedPool[S any, R any](qty int, src <-chan S, worker func(inputValue S, outputC chan<- R)) <-chan R {
	return Merge(NewPool(qty, src, worker)...)
}
