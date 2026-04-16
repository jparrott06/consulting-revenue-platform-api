package jobworker

import (
	"context"
	"time"
)

// DLQWriter records jobs that exhausted retries.
type DLQWriter interface {
	Write(ctx context.Context, queue string, payload []byte, attempt int, errText string) error
}

// RunWithRetry executes fn until it succeeds or maxAttempts is reached, applying exponential backoff between attempts.
// If all attempts fail and dlq is non-nil, the final error is written to the dead-letter sink.
func RunWithRetry(ctx context.Context, maxAttempts int, baseBackoff time.Duration, queue string, payload []byte, fn func(context.Context) error, dlq DLQWriter) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if baseBackoff <= 0 {
		baseBackoff = 50 * time.Millisecond
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if attempt == maxAttempts {
			break
		}

		wait := backoffAfterFailure(attempt, baseBackoff)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		timer.Stop()
	}

	if dlq != nil {
		if err := dlq.Write(ctx, queue, payload, maxAttempts, lastErr.Error()); err != nil {
			return err
		}
	}
	return lastErr
}

func backoffAfterFailure(attempt int, base time.Duration) time.Duration {
	d := base
	for i := 1; i < attempt; i++ {
		next := d * 2
		if next > 10*time.Second {
			return 10 * time.Second
		}
		d = next
	}
	return d
}
