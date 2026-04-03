package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/davehornigan/MovieTagger/internal/logging"
	"github.com/davehornigan/MovieTagger/internal/model"
)

func DoWithRetry[T any](
	ctx context.Context,
	logger logging.Logger,
	provider model.ProviderKind,
	operation string,
	attempts int,
	baseBackoff time.Duration,
	sleepFn func(time.Duration),
	fn func() (T, error),
) (T, error) {
	var zero T
	if attempts <= 0 {
		attempts = 1
	}
	if baseBackoff <= 0 {
		baseBackoff = 200 * time.Millisecond
	}
	if sleepFn == nil {
		sleepFn = time.Sleep
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		val, err := fn()
		if err == nil {
			return val, nil
		}

		lastErr = err
		if attempt < attempts {
			if logger != nil {
				logger.LogProviderRetry(provider, operation, attempt, err)
			}
			sleepFn(baseBackoff * time.Duration(attempt))
			continue
		}
	}

	if logger != nil {
		logger.Errorf("provider failure provider=%s operation=%q err=%v", provider, operation, lastErr)
	}
	return zero, fmt.Errorf("provider %s %s failed after %d attempts: %w", provider, operation, attempts, lastErr)
}
