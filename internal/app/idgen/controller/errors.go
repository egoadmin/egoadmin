package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, idgen.ErrInvalidConfig):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, idgen.ErrNameNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, idgen.ErrNameDisabled):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, idgen.ErrMachineLeaseLost):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, idgen.ErrMachineIDOverflow):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, idgen.ErrStoreUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, idgen.ErrSegmentConflict):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, fmt.Sprintf("idgen internal error: %v", err))
	}
}
