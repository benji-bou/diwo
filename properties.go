package diwo

import "context"

type listenProperty[E any] struct {
	value  E
	c      chan<- E
	ctx    context.Context
	broker *Broker[E]
}

func Listen[E any](initialProperty E) *listenProperty[E] {
	return ListenWithContext(context.Background(), initialProperty)
}

func ListenChan[E any](ctx context.Context, initialProperty E, src <-chan E) *listenProperty[E] {
	listenProp := ListenWithContext(ctx, initialProperty)
	listenProp.Forward(ctx, src)
	return listenProp
}

func ListenWithContext[E any](ctx context.Context, initialProperty E) *listenProperty[E] {
	c := make(chan E)
	lp := &listenProperty[E]{value: initialProperty, c: c, broker: NewBroker(c), ctx: ctx}
	return lp
}

func (lp listenProperty[E]) Value() E {
	return lp.value
}

func (lp *listenProperty[E]) Set(value E) {
	lp.value = value
	lp.c <- value
}

func (lp *listenProperty[E]) Forward(ctx context.Context, i <-chan E) {
	for elem := range i {
		lp.Set(elem)
	}
}

func (lp listenProperty[E]) Updates() <-chan E {
	return lp.broker.Subscribe()
}

func (lp listenProperty[E]) Close() {
	close(lp.c)
}
