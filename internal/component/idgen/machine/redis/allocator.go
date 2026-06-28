package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
	goredis "github.com/redis/go-redis/v9"
)

const defaultKeyPrefix = "idgen"

// Allocator coordinates process machine IDs with Redis leases.
type Allocator struct {
	client    evaler
	keyPrefix string
}

type evaler interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *goredis.Cmd
}

func New(client evaler, options ...Option) *Allocator {
	a := &Allocator{
		client:    client,
		keyPrefix: defaultKeyPrefix,
	}
	for _, option := range options {
		option(a)
	}
	return a
}

func (a *Allocator) Allocate(ctx context.Context, req idgen.MachineRequest) (idgen.MachineLease, error) {
	if a == nil || a.client == nil {
		return idgen.MachineLease{}, idgen.ErrStoreUnavailable
	}
	if req.Namespace == "" || req.InstanceID == "" || req.MaxMachineID < 0 || req.TTL <= 0 {
		return idgen.MachineLease{}, fmt.Errorf("%w: invalid machine request", idgen.ErrInvalidConfig)
	}
	sessionID, err := randomSessionID()
	if err != nil {
		return idgen.MachineLease{}, err
	}
	result, err := a.client.Eval(ctx, allocateScript, nil,
		a.keyPrefix,
		req.Namespace,
		req.InstanceID,
		sessionID,
		strconv.Itoa(req.MaxMachineID),
		strconv.FormatInt(req.TTL.Milliseconds(), 10),
	).Slice()
	if err != nil {
		return idgen.MachineLease{}, fmt.Errorf("allocate redis machine id: %w", err)
	}
	if len(result) < 1 {
		return idgen.MachineLease{}, idgen.ErrMachineIDOverflow
	}
	machineID, err := toInt(result[0])
	if err != nil {
		return idgen.MachineLease{}, err
	}
	if machineID < 0 {
		return idgen.MachineLease{}, idgen.ErrMachineIDOverflow
	}
	return idgen.MachineLease{
		Namespace:     req.Namespace,
		InstanceID:    req.InstanceID,
		SessionID:     sessionID,
		MachineID:     machineID,
		TTL:           req.TTL,
		RenewInterval: req.RenewInterval,
		ExpiresAt:     time.Now().Add(req.TTL),
	}, nil
}

func (a *Allocator) Renew(ctx context.Context, lease idgen.MachineLease) error {
	if a == nil || a.client == nil {
		return idgen.ErrStoreUnavailable
	}
	result, err := a.client.Eval(ctx, renewScript, nil,
		a.keyPrefix,
		lease.Namespace,
		lease.InstanceID,
		lease.SessionID,
		strconv.Itoa(lease.MachineID),
		strconv.FormatInt(lease.TTL.Milliseconds(), 10),
	).Int()
	if err != nil {
		return fmt.Errorf("renew redis machine id: %w", err)
	}
	if result != 1 {
		return idgen.ErrMachineLeaseLost
	}
	return nil
}

func (a *Allocator) Release(ctx context.Context, lease idgen.MachineLease) error {
	if a == nil || a.client == nil {
		return idgen.ErrStoreUnavailable
	}
	_, err := a.client.Eval(ctx, releaseScript, nil,
		a.keyPrefix,
		lease.Namespace,
		lease.InstanceID,
		lease.SessionID,
		strconv.Itoa(lease.MachineID),
	).Int()
	if err != nil {
		return fmt.Errorf("release redis machine id: %w", err)
	}
	return nil
}

func randomSessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate idgen session id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func toInt(v any) (int, error) {
	switch value := v.(type) {
	case int64:
		return int(value), nil
	case int:
		return value, nil
	case string:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected redis integer type %T", v)
	}
}
