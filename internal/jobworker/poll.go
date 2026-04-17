package jobworker

import (
	"context"
	"errors"
	"time"
)

// ErrIdle tells Poll there was no work to do; Poll will wait before the next tick.
var ErrIdle = errors.New("no work available")

// Poll invokes tick immediately, then every interval until ctx is cancelled.
// tick may return ErrIdle to indicate an empty queue; any other non-nil error stops Poll.
// A non-positive interval defaults to one second.
func Poll(ctx context.Context, interval time.Duration, tick func(context.Context) error) error {
	if interval <= 0 {
		interval = time.Second
	}
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}

		err := tick(ctx)
		if err != nil && !errors.Is(err, ErrIdle) {
			return err
		}

		timer.Reset(interval)
	}
}
