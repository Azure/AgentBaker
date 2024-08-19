package containerimage

import (
	"sync"

	"go.uber.org/multierr"
)

type puller func() error

func pullInParallel(pullers []puller, maxParallelism int) error {
	guard := make(chan struct{}, maxParallelism)
	errs := make([]error, len(pullers))
	wg := sync.WaitGroup{}

	for idx, p := range pullers {
		guard <- struct{}{}
		wg.Add(1)
		go func(p puller, idx int) {
			errs[idx] = p()
			<-guard
			wg.Done()
		}(p, idx)
	}

	// wait for any outstanding pullers to complete
	wg.Wait()

	var merr error
	for _, err := range errs {
		if err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	return merr
}
