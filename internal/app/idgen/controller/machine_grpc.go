package controller

import (
	"context"
	"time"

	idgenv1 "github.com/egoadmin/egoadmin/api/gen/go/idgen/v1"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *MachineLeaseGRPC) AllocateLease(ctx context.Context, in *idgenv1.AllocateLeaseRequest) (*idgenv1.AllocateLeaseResponse, error) {
	lease, err := s.usecase.Allocate(ctx, idgen.MachineRequest{
		Namespace:        in.GetNamespace(),
		InstanceID:       in.GetInstanceId(),
		StableInstanceID: in.GetStableInstanceId(),
		MaxMachineID:     int(in.GetMaxMachineId()),
		TTL:              seconds(in.GetTtlSeconds()),
		RenewInterval:    seconds(in.GetRenewIntervalSeconds()),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return leaseToRPC(lease), nil
}

func (s *MachineLeaseGRPC) RenewLease(ctx context.Context, in *idgenv1.RenewLeaseRequest) (*idgenv1.RenewLeaseResponse, error) {
	err := s.usecase.Renew(ctx, idgen.MachineLease{
		Namespace:     in.GetNamespace(),
		InstanceID:    in.GetInstanceId(),
		SessionID:     in.GetSessionId(),
		MachineID:     int(in.GetMachineId()),
		TTL:           seconds(in.GetTtlSeconds()),
		RenewInterval: seconds(in.GetRenewIntervalSeconds()),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &idgenv1.RenewLeaseResponse{}, nil
}

func (s *MachineLeaseGRPC) ReleaseLease(ctx context.Context, in *idgenv1.ReleaseLeaseRequest) (*idgenv1.ReleaseLeaseResponse, error) {
	err := s.usecase.Release(ctx, idgen.MachineLease{
		Namespace:  in.GetNamespace(),
		InstanceID: in.GetInstanceId(),
		SessionID:  in.GetSessionId(),
		MachineID:  int(in.GetMachineId()),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return &idgenv1.ReleaseLeaseResponse{}, nil
}

func seconds(v int64) time.Duration {
	return time.Duration(v) * time.Second
}

func leaseToRPC(lease idgen.MachineLease) *idgenv1.AllocateLeaseResponse {
	return &idgenv1.AllocateLeaseResponse{
		Namespace:            lease.Namespace,
		InstanceId:           lease.InstanceID,
		SessionId:            lease.SessionID,
		MachineId:            int32(lease.MachineID),
		TtlSeconds:           int64(lease.TTL.Seconds()),
		RenewIntervalSeconds: int64(lease.RenewInterval.Seconds()),
		ExpiresAt:            timestamppb.New(lease.ExpiresAt),
	}
}
