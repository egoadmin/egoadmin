package mysql

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/egoadmin/egoadmin/internal/app/idgen/domain/machine"
	"github.com/egoadmin/egoadmin/internal/component/idgen"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MachineLeaseRepository struct {
	db  platformmysql.MysqlInterface
	now func() time.Time
}

var _ machine.Repository = (*MachineLeaseRepository)(nil)

func NewMachineLeaseRepository(db platformmysql.MysqlInterface) *MachineLeaseRepository {
	return &MachineLeaseRepository{
		db:  db,
		now: time.Now,
	}
}

func (r *MachineLeaseRepository) Allocate(ctx context.Context, req idgen.MachineRequest) (idgen.MachineLease, error) {
	if req.Namespace == "" || req.InstanceID == "" || req.MaxMachineID < 0 || req.TTL <= 0 {
		return idgen.MachineLease{}, fmt.Errorf("%w: invalid machine request", idgen.ErrInvalidConfig)
	}
	sessionID, err := randomSessionID()
	if err != nil {
		return idgen.MachineLease{}, err
	}
	var lease idgen.MachineLease
	err = r.db.Transaction(ctx, func(ctx context.Context) error {
		db := r.db.WithTx(ctx)
		now := r.now()
		existing, err := r.findReusableInstanceLease(ctx, req, now)
		if err != nil {
			return err
		}
		if existing != nil {
			existing.SessionID = sessionID
			existing.TTLMillis = req.TTL.Milliseconds()
			existing.RenewMillis = req.RenewInterval.Milliseconds()
			existing.ExpiresAt = now.Add(req.TTL)
			existing.LastRenewedAt = now
			if err = r.updateLeaseRow(db, existing); err != nil {
				return err
			}
			lease = modelToLease(existing)
			return nil
		}
		for id := int32(0); id <= int32(req.MaxMachineID); id++ {
			row := machineLeaseModel{}
			err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("namespace = ? AND machine_id = ?", req.Namespace, id).
				First(&row).Error
			switch {
			case err == nil:
				if row.ExpiresAt.After(now) {
					continue
				}
				row.InstanceID = req.InstanceID
				row.SessionID = sessionID
				row.TTLMillis = req.TTL.Milliseconds()
				row.RenewMillis = req.RenewInterval.Milliseconds()
				row.ExpiresAt = now.Add(req.TTL)
				row.LastRenewedAt = now
				if err = r.updateLeaseRow(db, &row); err != nil {
					return err
				}
				lease = modelToLease(&row)
				return nil
			case isNotFound(err):
				row = machineLeaseModel{
					Namespace:     req.Namespace,
					MachineID:     id,
					InstanceID:    req.InstanceID,
					SessionID:     sessionID,
					TTLMillis:     req.TTL.Milliseconds(),
					RenewMillis:   req.RenewInterval.Milliseconds(),
					ExpiresAt:     now.Add(req.TTL),
					LastRenewedAt: now,
					CreatedAt:     now,
					UpdatedAt:     now,
				}
				if err = db.Create(&row).Error; err != nil {
					return err
				}
				lease = modelToLease(&row)
				return nil
			default:
				return err
			}
		}
		return idgen.ErrMachineIDOverflow
	})
	if err != nil {
		return idgen.MachineLease{}, err
	}
	return lease, nil
}

func (r *MachineLeaseRepository) Renew(ctx context.Context, lease idgen.MachineLease) error {
	if lease.Namespace == "" || lease.InstanceID == "" || lease.SessionID == "" || lease.MachineID < 0 || lease.TTL <= 0 {
		return fmt.Errorf("%w: invalid machine lease", idgen.ErrInvalidConfig)
	}
	return r.db.Transaction(ctx, func(ctx context.Context) error {
		db := r.db.WithTx(ctx)
		var row machineLeaseModel
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("namespace = ? AND machine_id = ?", lease.Namespace, lease.MachineID).
			First(&row).Error; err != nil {
			if isNotFound(err) {
				return idgen.ErrMachineLeaseLost
			}
			return err
		}
		now := r.now()
		if row.InstanceID != lease.InstanceID || row.SessionID != lease.SessionID || !row.ExpiresAt.After(now) {
			return idgen.ErrMachineLeaseLost
		}
		row.TTLMillis = lease.TTL.Milliseconds()
		row.RenewMillis = lease.RenewInterval.Milliseconds()
		row.ExpiresAt = now.Add(lease.TTL)
		row.LastRenewedAt = now
		return r.updateLeaseRow(db, &row)
	})
}

func (r *MachineLeaseRepository) Release(ctx context.Context, lease idgen.MachineLease) error {
	if lease.Namespace == "" || lease.InstanceID == "" || lease.SessionID == "" || lease.MachineID < 0 {
		return fmt.Errorf("%w: invalid machine lease", idgen.ErrInvalidConfig)
	}
	return r.db.Transaction(ctx, func(ctx context.Context) error {
		db := r.db.WithTx(ctx)
		result := db.
			Where("namespace = ? AND machine_id = ? AND instance_id = ? AND session_id = ?",
				lease.Namespace, lease.MachineID, lease.InstanceID, lease.SessionID).
			Delete(&machineLeaseModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return idgen.ErrMachineLeaseLost
		}
		return nil
	})
}

func (r *MachineLeaseRepository) CleanupExpired(ctx context.Context, before time.Time, limit int) (int64, error) {
	if before.IsZero() {
		return 0, fmt.Errorf("%w: cleanup before time is zero", idgen.ErrInvalidConfig)
	}
	if limit <= 0 {
		return 0, fmt.Errorf("%w: cleanup limit must be positive", idgen.ErrInvalidConfig)
	}
	type leaseKey struct {
		Namespace string
		MachineID int32
	}
	var keys []leaseKey
	db := r.db.WithTx(ctx)
	if err := db.Model(&machineLeaseModel{}).
		Select("namespace", "machine_id").
		Where("expires_at < ?", before).
		Order("expires_at ASC").
		Limit(limit).
		Find(&keys).Error; err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, nil
	}
	var deleted int64
	err := r.db.Transaction(ctx, func(ctx context.Context) error {
		db := r.db.WithTx(ctx)
		for _, key := range keys {
			result := db.
				Where("namespace = ? AND machine_id = ? AND expires_at < ?", key.Namespace, key.MachineID, before).
				Delete(&machineLeaseModel{})
			if result.Error != nil {
				return result.Error
			}
			deleted += result.RowsAffected
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

func (r *MachineLeaseRepository) findReusableInstanceLease(ctx context.Context, req idgen.MachineRequest, now time.Time) (*machineLeaseModel, error) {
	db := r.db.WithTx(ctx)
	var row machineLeaseModel
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("namespace = ? AND instance_id = ? AND expires_at > ?", req.Namespace, req.InstanceID, now).
		Order("machine_id ASC").
		First(&row).Error
	if err == nil {
		return &row, nil
	}
	if isNotFound(err) {
		return nil, nil
	}
	return nil, err
}

func (r *MachineLeaseRepository) updateLeaseRow(db *gorm.DB, row *machineLeaseModel) error {
	now := r.now()
	result := db.Model(&machineLeaseModel{}).
		Where("namespace = ? AND machine_id = ?", row.Namespace, row.MachineID).
		Updates(map[string]any{
			"instance_id":     row.InstanceID,
			"session_id":      row.SessionID,
			"ttl_millis":      row.TTLMillis,
			"renew_millis":    row.RenewMillis,
			"expires_at":      row.ExpiresAt,
			"last_renewed_at": row.LastRenewedAt,
			"updated_at":      now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return idgen.ErrMachineLeaseLost
	}
	row.UpdatedAt = now
	return nil
}

func modelToLease(row *machineLeaseModel) idgen.MachineLease {
	if row == nil {
		return idgen.MachineLease{}
	}
	return idgen.MachineLease{
		Namespace:     row.Namespace,
		InstanceID:    row.InstanceID,
		SessionID:     row.SessionID,
		MachineID:     int(row.MachineID),
		TTL:           time.Duration(row.TTLMillis) * time.Millisecond,
		RenewInterval: time.Duration(row.RenewMillis) * time.Millisecond,
		ExpiresAt:     row.ExpiresAt,
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func randomSessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate idgen session id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
