package idgenclient

import (
	"context"
	"errors"
	"testing"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNormalizeErrorMapsGRPCCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "invalid argument", err: status.Error(codes.InvalidArgument, "bad"), want: idgen.ErrInvalidConfig},
		{name: "not found", err: status.Error(codes.NotFound, "missing"), want: idgen.ErrNameNotFound},
		{name: "disabled", err: status.Error(codes.FailedPrecondition, "name disabled"), want: idgen.ErrNameDisabled},
		{name: "lease lost", err: status.Error(codes.FailedPrecondition, "lease lost"), want: idgen.ErrMachineLeaseLost},
		{name: "exhausted", err: status.Error(codes.ResourceExhausted, "overflow"), want: idgen.ErrMachineIDOverflow},
		{name: "unavailable", err: status.Error(codes.Unavailable, "down"), want: idgen.ErrStoreUnavailable},
		{name: "deadline", err: status.Error(codes.DeadlineExceeded, "timeout"), want: idgen.ErrStoreUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeError(tt.err); !errors.Is(got, tt.want) {
				t.Fatalf("normalizeError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeErrorPreservesContextErrors(t *testing.T) {
	t.Parallel()

	if got := normalizeError(context.Canceled); !errors.Is(got, context.Canceled) {
		t.Fatalf("canceled = %v, want context.Canceled", got)
	}
	if got := normalizeError(context.DeadlineExceeded); !errors.Is(got, context.DeadlineExceeded) {
		t.Fatalf("deadline = %v, want context.DeadlineExceeded", got)
	}
}
