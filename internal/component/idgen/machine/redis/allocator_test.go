package redis

import (
	"context"
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
	goredis "github.com/redis/go-redis/v9"
)

func TestAllocator_AllocateRenewRelease(t *testing.T) {
	client := &fakeEvaler{}
	allocator := New(client, WithKeyPrefix("test:idgen"))
	req := idgen.MachineRequest{
		Namespace:     "test",
		InstanceID:    "instance-1",
		MaxMachineID:  3,
		TTL:           time.Minute,
		RenewInterval: time.Second,
	}
	lease, err := allocator.Allocate(context.Background(), req)
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if lease.MachineID != 0 {
		t.Fatalf("machine id = %d, want 0", lease.MachineID)
	}
	if err = allocator.Renew(context.Background(), lease); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if err = allocator.Release(context.Background(), lease); err != nil {
		t.Fatalf("release: %v", err)
	}
}

func BenchmarkMachineLeaseRedis(b *testing.B) {
	client := &fakeEvaler{}
	allocator := New(client, WithKeyPrefix("bench:idgen"))
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		lease, err := allocator.Allocate(ctx, idgen.MachineRequest{
			Namespace:     "bench",
			InstanceID:    "instance",
			MaxMachineID:  1023,
			TTL:           time.Minute,
			RenewInterval: time.Second,
		})
		if err != nil {
			b.Fatal(err)
		}
		if err = allocator.Renew(ctx, lease); err != nil {
			b.Fatal(err)
		}
	}
}

type fakeEvaler struct{}

func (f *fakeEvaler) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *goredis.Cmd {
	switch script {
	case allocateScript:
		return goredis.NewCmdResult([]interface{}{int64(0), args[3]}, nil)
	case renewScript:
		return goredis.NewCmdResult(int64(1), nil)
	case releaseScript:
		return goredis.NewCmdResult(int64(1), nil)
	default:
		return goredis.NewCmdResult(nil, nil)
	}
}
