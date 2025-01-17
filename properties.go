package diwo

import "context"

type Listen[E any] struct {
	value  E
	c      chan<- E
	ctx    context.Context
	broker *Broker[E]
}

func ListenProperty[E any](initialProperty E) *Listen[E] {
	return ListenWithContext(context.Background(), initialProperty)
}

func ListenChan[E any](ctx context.Context, initialProperty E, src <-chan E) *Listen[E] {
	listenProp := ListenWithContext(ctx, initialProperty)
	listenProp.Forward(ctx, src)
	return listenProp
}

func ListenWithContext[E any](ctx context.Context, initialProperty E) *Listen[E] {
	c := make(chan E)
	lp := &Listen[E]{value: initialProperty, c: c, broker: NewBroker(c), ctx: ctx}
	return lp
}

func (lp Listen[E]) Value() E {
	return lp.value
}

func (lp *Listen[E]) Set(value E) {
	lp.value = value
	lp.c <- value
}

func (lp *Listen[E]) Forward(ctx context.Context, i <-chan E) {
	for elem := range i {
		lp.Set(elem)
	}
}

func (lp Listen[E]) Updates() <-chan E {
	return lp.broker.Subscribe()
}

func (lp Listen[E]) Close() {
	close(lp.c)
}
