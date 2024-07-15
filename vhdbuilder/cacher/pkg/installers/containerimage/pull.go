package containerimage

import (
	"sync"

	"go.uber.org/multierr"
)

type puller func() error

func pullInParallel(pullers []puller, maxParallelism int) error {
	guard := make(chan struct{}, maxParallelism)
	wg := sync.WaitGroup{}
	errs := make([]error, len(pullers))

	for idx, p := range pullers {
		guard <- struct{}{}
		wg.Add(1)
		go func(p puller, idx int) {
			if err := p(); err != nil {
				errs[idx] = err
			}
			<-guard
			wg.Done()
		}(p, idx)
	}

	var merr error
	for _, err := range errs {
		if err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	return merr
}
