package toolkit

import (
	"context"
	"fmt"
	"sync"
)

// Memoize caches results, including errors, for each key for the returned function's lifetime.
// Waiters can cancel without canceling the operation started by the first caller.
func Memoize[Request comparable, Response any](fn func(context.Context, Request) (Response, error)) func(context.Context, Request) (Response, error) {
	type entry struct {
		done  chan struct{}
		value Response
		err   error
	}
	var cache sync.Map
	return func(ctx context.Context, key Request) (Response, error) {
		actual, loaded := cache.LoadOrStore(key, &entry{done: make(chan struct{})})
		e := actual.(*entry)
		if !loaded {
			defer close(e.done)
			defer func() {
				if recovered := recover(); recovered != nil {
					e.err = fmt.Errorf("cached operation panicked: %v", recovered)
					panic(recovered)
				}
			}()
			e.value, e.err = fn(ctx, key)
			return e.value, e.err
		}
		select {
		case <-e.done:
			return e.value, e.err
		case <-ctx.Done():
			var zero Response
			return zero, ctx.Err()
		}
	}
}
