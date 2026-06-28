package gormstore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/idgen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store allocates idgen segments through GORM transactions.
type Store struct {
	db        *gorm.DB
	tableName string
	now       func() time.Time
}

func New(db *gorm.DB, options ...Option) *Store {
	s := &Store{
		db:        db,
		tableName: DefaultTableName,
		now:       time.Now,
	}
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *Store) Ensure(ctx context.Context, namespace string, name string, cfg idgen.EnsureSegmentConfig) error {
	if s == nil || s.db == nil {
		return idgen.ErrStoreUnavailable
	}
	if namespace == "" || name == "" {
		return fmt.Errorf("%w: namespace and name are required", idgen.ErrInvalidConfig)
	}
	cfg = normalizeEnsureConfig(cfg)
	if cfg.Status < 0 || cfg.Status > math.MaxInt32 {
		return fmt.Errorf("%w: invalid segment status", idgen.ErrInvalidConfig)
	}
	now := s.now()
	return s.db.WithContext(ctx).
		Table(s.tableName).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&SegmentModel{
			Namespace:   namespace,
			Name:        name,
			NextID:      cfg.NextID,
			Step:        cfg.Step,
			MinStep:     cfg.MinStep,
			MaxStep:     cfg.MaxStep,
			Status:      int32(cfg.Status),
			Description: cfg.Description,
			CreatedAt:   now,
			UpdatedAt:   now,
		}).Error
}

func (s *Store) Fetch(ctx context.Context, namespace string, name string, step int64) (allocated idgen.Range, cfg idgen.SegmentConfig, err error) {
	if s == nil || s.db == nil {
		return idgen.Range{}, idgen.SegmentConfig{}, idgen.ErrStoreUnavailable
	}
	if namespace == "" || name == "" {
		return idgen.Range{}, idgen.SegmentConfig{}, fmt.Errorf("%w: namespace and name are required", idgen.ErrInvalidConfig)
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row SegmentModel
		if er := tx.Table(s.tableName).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("namespace = ? AND name = ?", namespace, name).
			First(&row).Error; er != nil {
			if errors.Is(er, gorm.ErrRecordNotFound) {
				return idgen.ErrNameNotFound
			}
			return er
		}
		if row.Status != StatusEnabled {
			return idgen.ErrNameDisabled
		}
		if row.NextID < 0 || row.Step <= 0 {
			return fmt.Errorf("%w: invalid idgen segment row", idgen.ErrInvalidConfig)
		}
		actualStep := step
		if actualStep <= 0 {
			actualStep = row.Step
		}
		minStep := row.MinStep
		if minStep <= 0 {
			minStep = row.Step
		}
		maxStep := row.MaxStep
		if maxStep <= 0 {
			maxStep = row.Step
		}
		if actualStep < minStep {
			actualStep = minStep
		}
		if actualStep > maxStep {
			actualStep = maxStep
		}
		if row.NextID > math.MaxInt64-actualStep {
			return idgen.ErrOverflow
		}
		start := row.NextID
		end := start + actualStep
		now := s.now()
		result := tx.Table(s.tableName).
			Where("namespace = ? AND name = ? AND next_id = ?", namespace, name, row.NextID).
			Updates(map[string]any{
				"next_id":       end,
				"last_step":     actualStep,
				"last_fetch_at": &now,
				"updated_at":    now,
			})
		if er := result.Error; er != nil {
			return er
		}
		if result.RowsAffected != 1 {
			return idgen.ErrSegmentConflict
		}
		allocated = idgen.Range{Start: start, End: end}
		cfg = idgen.SegmentConfig{
			Step:    row.Step,
			MinStep: minStep,
			MaxStep: maxStep,
			Status:  idgen.SegmentStatusEnabled,
		}
		return nil
	})
	if err != nil {
		return idgen.Range{}, idgen.SegmentConfig{}, err
	}
	return allocated, cfg, nil
}

func normalizeEnsureConfig(cfg idgen.EnsureSegmentConfig) idgen.EnsureSegmentConfig {
	if cfg.NextID <= 0 {
		cfg.NextID = 1
	}
	defaults := idgen.DefaultConfig()
	if cfg.Step <= 0 {
		cfg.Step = defaults.Step
	}
	if cfg.MinStep <= 0 {
		cfg.MinStep = defaults.MinStep
	}
	if cfg.MaxStep <= 0 {
		cfg.MaxStep = defaults.MaxStep
	}
	if cfg.MinStep > cfg.Step {
		cfg.Step = cfg.MinStep
	}
	if cfg.MaxStep < cfg.Step {
		cfg.MaxStep = cfg.Step
	}
	if cfg.Status == 0 {
		cfg.Status = idgen.SegmentStatusEnabled
	}
	return cfg
}

func (s *Store) Health(ctx context.Context) error {
	if s == nil || s.db == nil {
		return idgen.ErrStoreUnavailable
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
