package idgenclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	idgenv1 "github.com/egoadmin/egoadmin/api/gen/go/idgen/v1"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
	"github.com/egoadmin/egoadmin/internal/platform/discovery"
	"github.com/google/wire"
	"github.com/gotomicro/ego/client/egrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var ProviderSet = wire.NewSet(
	NewClient,
	NewSegmentService,
	NewMachineLeaseService,
)

type SegmentService interface {
	Ensure(ctx context.Context, namespace string, name string, cfg idgen.EnsureSegmentConfig) error
	Allocate(ctx context.Context, namespace string, name string, requestedStep int64) (idgen.Range, idgen.SegmentConfig, error)
	Health(ctx context.Context) error
}

type MachineLeaseService interface {
	Allocate(ctx context.Context, req idgen.MachineRequest) (idgen.MachineLease, error)
	Renew(ctx context.Context, lease idgen.MachineLease) error
	Release(ctx context.Context, lease idgen.MachineLease) error
}

type Client struct {
	conn         *egrpc.Component
	Segment      SegmentService
	MachineLease MachineLeaseService
}

func NewClient(_ discovery.Ready) *Client {
	conn := egrpc.Load("client.grpc.idgen").Build()
	return &Client{
		conn:         conn,
		Segment:      &segmentClient{client: idgenv1.NewSegmentServiceClient(conn.ClientConn)},
		MachineLease: &machineLeaseClient{client: idgenv1.NewMachineLeaseServiceClient(conn.ClientConn)},
	}
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil || c.conn.ClientConn == nil {
		return nil
	}
	return c.conn.Close()
}

func NewSegmentService(client *Client) SegmentService {
	return client.Segment
}

func NewMachineLeaseService(client *Client) MachineLeaseService {
	return client.MachineLease
}

var forwardedMetadataKeys = map[string]struct{}{
	"authorization":     {},
	"x-forwarded-for":   {},
	"x-forwarded-host":  {},
	"x-request-id":      {},
	"x-correlation-id":  {},
	"x-b3-traceid":      {},
	"x-b3-spanid":       {},
	"x-b3-parentspanid": {},
	"x-b3-sampled":      {},
	"x-b3-flags":        {},
	"traceparent":       {},
	"tracestate":        {},
	"grpc-trace-bin":    {},
	"uber-trace-id":     {},
	"jaeger-debug-id":   {},
	"jaeger-baggage":    {},
	"baggage":           {},
}

func outgoingContext(ctx context.Context) context.Context {
	incoming, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	outgoing, _ := metadata.FromOutgoingContext(ctx)
	merged := outgoing.Copy()
	for key, values := range incoming {
		key = strings.ToLower(key)
		if _, ok := forwardedMetadataKeys[key]; !ok {
			continue
		}
		merged[key] = values
	}
	return metadata.NewOutgoingContext(ctx, merged)
}

type segmentClient struct {
	client idgenv1.SegmentServiceClient
}

func (c *segmentClient) Ensure(ctx context.Context, namespace string, name string, cfg idgen.EnsureSegmentConfig) error {
	_, err := c.client.EnsureSegment(outgoingContext(ctx), &idgenv1.EnsureSegmentRequest{
		Namespace:   namespace,
		Name:        name,
		NextId:      cfg.NextID,
		Step:        cfg.Step,
		MinStep:     cfg.MinStep,
		MaxStep:     cfg.MaxStep,
		Status:      int32(cfg.Status),
		Description: cfg.Description,
	})
	return normalizeError(err)
}

func (c *segmentClient) Allocate(ctx context.Context, namespace string, name string, requestedStep int64) (idgen.Range, idgen.SegmentConfig, error) {
	out, err := c.client.AllocateSegment(outgoingContext(ctx), &idgenv1.AllocateSegmentRequest{
		Namespace:     namespace,
		Name:          name,
		RequestedStep: requestedStep,
	})
	if err != nil {
		return idgen.Range{}, idgen.SegmentConfig{}, normalizeError(err)
	}
	return idgen.Range{
			Start: out.GetStart(),
			End:   out.GetEnd(),
		}, idgen.SegmentConfig{
			Step:    out.GetStep(),
			MinStep: out.GetMinStep(),
			MaxStep: out.GetMaxStep(),
			Status:  int(out.GetStatus()),
		}, nil
}

func (c *segmentClient) Health(ctx context.Context) error {
	_, err := c.client.Health(outgoingContext(ctx), &idgenv1.SegmentServiceHealthRequest{})
	return normalizeError(err)
}

type machineLeaseClient struct {
	client idgenv1.MachineLeaseServiceClient
}

func (c *machineLeaseClient) Allocate(ctx context.Context, req idgen.MachineRequest) (idgen.MachineLease, error) {
	out, err := c.client.AllocateLease(outgoingContext(ctx), &idgenv1.AllocateLeaseRequest{
		Namespace:            req.Namespace,
		InstanceId:           req.InstanceID,
		StableInstanceId:     req.StableInstanceID,
		MaxMachineId:         int32(req.MaxMachineID),
		TtlSeconds:           int64(req.TTL.Seconds()),
		RenewIntervalSeconds: int64(req.RenewInterval.Seconds()),
	})
	if err != nil {
		return idgen.MachineLease{}, normalizeError(err)
	}
	return machineLeaseFromRPC(out), nil
}

func (c *machineLeaseClient) Renew(ctx context.Context, lease idgen.MachineLease) error {
	_, err := c.client.RenewLease(outgoingContext(ctx), &idgenv1.RenewLeaseRequest{
		Namespace:            lease.Namespace,
		InstanceId:           lease.InstanceID,
		SessionId:            lease.SessionID,
		MachineId:            int32(lease.MachineID),
		TtlSeconds:           int64(lease.TTL.Seconds()),
		RenewIntervalSeconds: int64(lease.RenewInterval.Seconds()),
	})
	return normalizeError(err)
}

func (c *machineLeaseClient) Release(ctx context.Context, lease idgen.MachineLease) error {
	_, err := c.client.ReleaseLease(outgoingContext(ctx), &idgenv1.ReleaseLeaseRequest{
		Namespace:  lease.Namespace,
		InstanceId: lease.InstanceID,
		SessionId:  lease.SessionID,
		MachineId:  int32(lease.MachineID),
	})
	return normalizeError(err)
}

func machineLeaseFromRPC(out *idgenv1.AllocateLeaseResponse) idgen.MachineLease {
	if out == nil {
		return idgen.MachineLease{}
	}
	return idgen.MachineLease{
		Namespace:     out.GetNamespace(),
		InstanceID:    out.GetInstanceId(),
		SessionID:     out.GetSessionId(),
		MachineID:     int(out.GetMachineId()),
		TTL:           time.Duration(out.GetTtlSeconds()) * time.Second,
		RenewInterval: time.Duration(out.GetRenewIntervalSeconds()) * time.Second,
		ExpiresAt:     out.GetExpiresAt().AsTime(),
	}
}

func Timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return fmt.Errorf("%w: %s", idgen.ErrInvalidConfig, st.Message())
	case codes.NotFound:
		return idgen.ErrNameNotFound
	case codes.FailedPrecondition:
		if strings.Contains(st.Message(), "lease") {
			return idgen.ErrMachineLeaseLost
		}
		return idgen.ErrNameDisabled
	case codes.ResourceExhausted:
		return idgen.ErrMachineIDOverflow
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf("%w: %s", idgen.ErrStoreUnavailable, st.Message())
	default:
		return err
	}
}
