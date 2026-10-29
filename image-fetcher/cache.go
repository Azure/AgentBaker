package main

import (
	"context"
	"fmt"
	"os"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"golang.org/x/sync/errgroup"
)

func cacheImages(ctx context.Context, client *containerd.Client, refs []string, concurrency int, writeEntry func(string) error) error {
	if concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}
	jobs := make(chan string, len(refs))
	for _, ref := range refs {
		jobs <- ref
	}
	close(jobs)

	group, groupCtx := errgroup.WithContext(ctx)
	for range min(concurrency, len(refs)) {
		group.Go(func() error {
			for ref := range jobs {
				if err := groupCtx.Err(); err != nil {
					return err
				}
				if err := fetchWithRetry(groupCtx, client, ref); err != nil {
					return fmt.Errorf("cache %s: %w", ref, err)
				}
				if err := writeEntry(ref); err != nil {
					return err
				}
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return ctx.Err()
}

func fetchWithRetry(ctx context.Context, client *containerd.Client, ref string) error {
	const maxAttempts = 10
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		err := fetchImage(attemptCtx, client, ref)
		if err == nil {
			err = attemptCtx.Err()
		}
		cancel()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt == maxAttempts {
			return fmt.Errorf("failed after %d attempts: %w", attempt, err)
		}
		fmt.Fprintf(os.Stderr, "RETRY %s (attempt %d/%d): %v\n", ref, attempt, maxAttempts, err)
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
