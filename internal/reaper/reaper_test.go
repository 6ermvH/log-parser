//go:build !integration

package reaper

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeRepo struct {
	calls atomic.Int64
	count int
	err   error
}

func (f *fakeRepo) ReapStaleProcessing(_ context.Context, _ time.Duration) (int, error) {
	f.calls.Add(1)

	return f.count, f.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestReaper_CallsRepoOnTicks(t *testing.T) {
	t.Parallel()

	fr := &fakeRepo{}
	r := New(fr, 10*time.Millisecond, time.Minute, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	time.Sleep(45 * time.Millisecond)
	cancel()
	<-done

	assert.GreaterOrEqual(t, fr.calls.Load(), int64(2))
}

func TestReaper_ContinuesAfterError(t *testing.T) {
	t.Parallel()

	fr := &fakeRepo{err: errors.New("transient failure")}
	r := New(fr, 10*time.Millisecond, time.Minute, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	time.Sleep(45 * time.Millisecond)
	cancel()
	<-done

	assert.GreaterOrEqual(t, fr.calls.Load(), int64(2))
}

func TestReaper_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	fr := &fakeRepo{}
	r := New(fr, 10*time.Millisecond, time.Minute, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("reaper did not stop within timeout")
	}
}
