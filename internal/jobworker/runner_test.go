package jobworker

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memDLQ struct {
	calls   int
	lastErr string
}

func (m *memDLQ) Write(_ context.Context, _ string, _ []byte, _ int, errText string) error {
	m.calls++
	m.lastErr = errText
	return nil
}

func TestRunWithRetry_SucceedsAfterFailures(t *testing.T) {
	ctx := context.Background()
	var calls int
	dlq := &memDLQ{}
	err := RunWithRetry(ctx, 4, time.Millisecond, "payments", []byte(`{}`), func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	}, dlq)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
	if dlq.calls != 0 {
		t.Fatalf("expected no DLQ writes, got %d", dlq.calls)
	}
}

func TestRunWithRetry_DeadLettersAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	dlq := &memDLQ{}
	err := RunWithRetry(ctx, 3, time.Millisecond, "webhooks", []byte(`{"id":1}`), func(context.Context) error {
		return errors.New("poison")
	}, dlq)
	if err == nil || err.Error() != "poison" {
		t.Fatalf("expected poison error, got %v", err)
	}
	if dlq.calls != 1 {
		t.Fatalf("expected 1 DLQ write, got %d", dlq.calls)
	}
	if dlq.lastErr != "poison" {
		t.Fatalf("unexpected dlq error text: %q", dlq.lastErr)
	}
}

func TestRunWithRetry_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dlq := &memDLQ{}
	err := RunWithRetry(ctx, 5, time.Millisecond, "q", []byte(`{}`), func(context.Context) error {
		return errors.New("nope")
	}, dlq)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
	if dlq.calls != 0 {
		t.Fatalf("expected no DLQ on cancel, got %d", dlq.calls)
	}
}
