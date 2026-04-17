package jobworker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPoll_FirstTickImmediate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var calls int
	err := Poll(ctx, 50*time.Millisecond, func(context.Context) error {
		calls++
		return ErrIdle
	})
	if err == nil || (!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)) {
		t.Fatalf("expected ctx cancel or deadline, got %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected at least 2 ticks, got %d", calls)
	}
}

func TestPoll_ReturnsTickError(t *testing.T) {
	ctx := context.Background()
	err := Poll(ctx, time.Millisecond, func(context.Context) error {
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected errBoom, got %v", err)
	}
}

var errBoom = errors.New("boom")
