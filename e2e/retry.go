package e2e_test

import (
	"fmt"
	"testing"
	"time"
)

func doWithRetry(funcToRetry func() error, numRetries int, waitInterval time.Duration, t *testing.T) error {
	if numRetries <= 0 {
		return fmt.Errorf("specified number of retries must > 0, got: %d", numRetries)
	}
	var err error
	for i := 0; i < numRetries; i++ {
		err = funcToRetry()
		if err == nil {
			return nil
		}

		if i < numRetries-1 {
			t.Logf("encountered an error while performing operation: %q, will retry...", err)
			time.Sleep(waitInterval)
		}
	}

	return err
}
