package shutdown

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/platform/health"
)

func TestManagerBeforeStopMarksNotReadyAndDrains(t *testing.T) {
	t.Parallel()

	h := &health.Options{}
	h.Ready()
	manager := NewManager(Config{
		StopTimeout:  time.Second,
		DrainTimeout: 10 * time.Millisecond,
		CloseTimeout: time.Second,
	}, h, nil)

	start := time.Now()
	if err := manager.beforeStop(); err != nil {
		t.Fatalf("beforeStop() error = %v", err)
	}
	if h.IsReady() {
		t.Fatalf("health ready = true, want false")
	}
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Fatalf("beforeStop elapsed = %s, want drain delay", elapsed)
	}
}

func TestManagerAfterStopClosesResourcesInReverseOrder(t *testing.T) {
	t.Parallel()

	var got []string
	manager := NewManager(Config{
		StopTimeout:  time.Second,
		DrainTimeout: time.Millisecond,
		CloseTimeout: time.Second,
	}, nil, nil)
	manager.Register("first", func(context.Context) error {
		got = append(got, "first")
		return nil
	})
	manager.Register("second", func(context.Context) error {
		got = append(got, "second")
		return nil
	})

	if err := manager.afterStop(); err != nil {
		t.Fatalf("afterStop() error = %v", err)
	}
	want := []string{"second", "first"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("close order = %#v, want %#v", got, want)
	}
}

func TestManagerAfterStopAggregatesCloseErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	manager := NewManager(Config{
		StopTimeout:  time.Second,
		DrainTimeout: time.Millisecond,
		CloseTimeout: time.Second,
	}, nil, nil)
	manager.Register("broken", func(context.Context) error {
		return boom
	})

	err := manager.afterStop()
	if !errors.Is(err, boom) {
		t.Fatalf("afterStop() error = %v, want wrapped boom", err)
	}
}
