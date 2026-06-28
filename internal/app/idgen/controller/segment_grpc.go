package controller

import (
	"context"

	idgenv1 "github.com/egoadmin/egoadmin/api/gen/go/idgen/v1"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
)

func (s *SegmentGRPC) EnsureSegment(ctx context.Context, in *idgenv1.EnsureSegmentRequest) (*idgenv1.EnsureSegmentResponse, error) {
	err := s.usecase.Ensure(ctx, in.GetNamespace(), in.GetName(), idgen.EnsureSegmentConfig{
		NextID:      in.GetNextId(),
		Step:        in.GetStep(),
		MinStep:     in.GetMinStep(),
		MaxStep:     in.GetMaxStep(),
		Status:      int(in.GetStatus()),
		Description: in.GetDescription(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &idgenv1.EnsureSegmentResponse{}, nil
}

func (s *SegmentGRPC) AllocateSegment(ctx context.Context, in *idgenv1.AllocateSegmentRequest) (*idgenv1.AllocateSegmentResponse, error) {
	r, cfg, err := s.usecase.Allocate(ctx, in.GetNamespace(), in.GetName(), in.GetRequestedStep())
	if err != nil {
		return nil, mapError(err)
	}
	return &idgenv1.AllocateSegmentResponse{
		Start:   r.Start,
		End:     r.End,
		Step:    cfg.Step,
		MinStep: cfg.MinStep,
		MaxStep: cfg.MaxStep,
		Status:  int32(cfg.Status),
	}, nil
}

func (s *SegmentGRPC) Health(ctx context.Context, _ *idgenv1.SegmentServiceHealthRequest) (*idgenv1.SegmentServiceHealthResponse, error) {
	if err := s.usecase.Health(ctx); err != nil {
		return nil, mapError(err)
	}
	return &idgenv1.SegmentServiceHealthResponse{}, nil
}
