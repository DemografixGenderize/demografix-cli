package batch

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/DemografixGenderize/demografix-cli/internal/api"
)

// Outcome is the result for one item, placed at outcomes[Item.Index]. Done is
// true only when the item's request succeeded; an item whose request was never
// scheduled (e.g. cancelled after a 429) has Done false and Err nil.
type Outcome[T any] struct {
	Item  Item
	Value T
	Done  bool
	Err   error
}

// RunRequests runs fn for each request with at most concurrency in flight,
// placing each item's result at outcomes[Item.Index] so input order is
// preserved. A 429 cancels remaining scheduling; other errors do not abort the
// run. The returned Quota is the freshest view (smallest Remaining seen) and the
// returned error is the first error encountered, if any.
func RunRequests[T any](
	ctx context.Context,
	reqs []Request,
	total int,
	concurrency int,
	fn func(context.Context, Request) ([]T, api.Quota, error),
) ([]Outcome[T], api.Quota, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if concurrency < 1 {
		concurrency = 1
	}

	outcomes := make([]Outcome[T], total)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var quota api.Quota
	var firstErr error

	for _, r := range reqs {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(r Request) {
			defer wg.Done()
			defer func() { <-sem }()

			vals, q, err := fn(ctx, r)
			// A response whose length does not match the request makes
			// positional recovery untrustworthy for every item; treat the
			// whole request as failed rather than marking trailing items Done
			// with a zero value.
			if err == nil && len(vals) != len(r.Items) {
				err = fmt.Errorf("api returned %d predictions for %d names", len(vals), len(r.Items))
			}

			mu.Lock()
			defer mu.Unlock()

			if q.Limit != 0 && (quota.Limit == 0 || q.Remaining < quota.Remaining) {
				quota = q
			}
			for j, it := range r.Items {
				outcomes[it.Index].Item = it
				if err != nil {
					outcomes[it.Index].Err = err
					continue
				}
				outcomes[it.Index].Done = true
				outcomes[it.Index].Value = vals[j]
			}
			if err != nil {
				var rle *api.RateLimitError
				if errors.As(err, &rle) {
					cancel()
					// A 429 is the actionable error for recovery, so prefer it
					// over any earlier non-429 error recorded under concurrency.
					var existing *api.RateLimitError
					if firstErr == nil || !errors.As(firstErr, &existing) {
						firstErr = err
					}
				} else if firstErr == nil {
					firstErr = err
				}
			}
		}(r)
	}

	wg.Wait()
	return outcomes, quota, firstErr
}
