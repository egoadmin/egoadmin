package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xflake"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

var errUserRepositoryMethodNotMigrated = errors.New("user mysql repository: method not migrated")

// UserRepository implements user.Repository with MySQL.
type UserRepository struct {
	db    platformmysql.MysqlInterface
	idgen xflake.Geter
}

var _ userdomain.Repository = (*UserRepository)(nil)

// NewUserRepository creates a MySQL-backed user repository.
func NewUserRepository(db platformmysql.MysqlInterface, idgen xflake.Geter) *UserRepository {
	return &UserRepository{
		db:    db,
		idgen: idgen,
	}
}

func (r *UserRepository) NextID(ctx context.Context) (uint64, error) {
	id, err := r.idgen.Get()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *UserRepository) Create(ctx context.Context, aggregate *userdomain.User) error {
	model := userModelFromDomain(aggregate)
	if model == nil {
		return userdomain.ErrNotFound
	}
	if err := r.db.WithTx(ctx).Omit("Roles.*").Create(model).Error; err != nil {
		return err
	}
	aggregate.ID = model.ID
	return nil
}

func (r *UserRepository) Save(ctx context.Context, aggregate *userdomain.User) error {
	return fmt.Errorf("save user %d: %w", aggregate.ID, errUserRepositoryMethodNotMigrated)
}

func (r *UserRepository) Update(ctx context.Context, id uint64, aggregate *userdomain.User) error {
	model := userUpdateModelFromDomain(aggregate)
	if model == nil {
		return userdomain.ErrNotFound
	}
	db := r.db.WithTx(ctx)
	if err := replaceUserRoleJoins(db, id, aggregate.RoleIDs); err != nil {
		return err
	}
	return db.
		Omit("Roles").
		Model(&userModel{Model: xorm.Model{ID: id}}).
		Updates(model).
		Error
}

func replaceUserRoleJoins(db *gorm.DB, userID uint64, roleIDs []uint64) error {
	if err := db.Where(map[string]any{"user_model_id": userID}).Delete(&userRoleModel{}).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(roleIDs))
	joins := make([]userRoleModel, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		if roleID == 0 {
			continue
		}
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		joins = append(joins, userRoleModel{
			UserModelID: userID,
			RoleModelID: roleID,
		})
	}
	if len(joins) == 0 {
		return nil
	}
	return db.CreateInBatches(joins, 100).Error
}

func (r *UserRepository) FindByID(ctx context.Context, id uint64) (*userdomain.User, error) {
	model := &userModel{}
	err := r.db.WithTx(ctx).Preload("Roles").First(model, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, userdomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*userdomain.User, error) {
	model := &userModel{}
	err := r.db.WithTx(ctx).Preload("Roles").Where(&userModel{Username: username}).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, userdomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *UserRepository) FindByPhone(ctx context.Context, phone string) (*userdomain.User, error) {
	model := &userModel{}
	err := r.db.WithTx(ctx).Preload("Roles").Where(&userModel{Phone: &phone}).First(model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, userdomain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *UserRepository) List(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	return nil, 0, fmt.Errorf("list users: %w", errUserRepositoryMethodNotMigrated)
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id uint64, passwordHash string) error {
	return r.db.WithTx(ctx).
		Model(&userModel{Model: xorm.Model{ID: id}}).
		UpdateColumn("password", passwordHash).
		Error
}

func (r *UserRepository) MarkLoggedIn(ctx context.Context, id uint64, at time.Time, ip string) error {
	return r.db.WithTx(ctx).
		Model(&userModel{Model: xorm.Model{ID: id}}).
		UpdateColumns(map[string]any{
			"user_online":    int32(userdomain.OnlineStatusOnline),
			"heartbeat_time": at,
			"last_login_at":  at,
			"last_login_ip":  ip,
		}).
		Error
}

func (r *UserRepository) MarkOnline(ctx context.Context, id uint64, at time.Time) error {
	return r.db.WithTx(ctx).
		Session(&gorm.Session{SkipDefaultTransaction: true}).
		Model(&userModel{Model: xorm.Model{ID: id}}).
		UpdateColumns(map[string]any{
			"user_online":    int32(userdomain.OnlineStatusOnline),
			"heartbeat_time": at,
		}).
		Error
}

func (r *UserRepository) MarkOffline(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithTx(ctx).
		Model(&userModel{}).
		Where("id IN (?)", ids).
		UpdateColumn("user_online", int32(userdomain.OnlineStatusOffline)).
		Error
}

func (r *UserRepository) FindHeartbeatExpiredIDs(ctx context.Context, before time.Time) ([]uint64, error) {
	uids := make([]uint64, 0)
	err := r.db.WithTx(ctx).
		Model(&userModel{}).
		Where("heartbeat_time < ?", before).
		Where("user_online = ?", int32(userdomain.OnlineStatusOnline)).
		Pluck("id", &uids).
		Error
	if err != nil {
		return nil, err
	}
	return uids, nil
}

func (r *UserRepository) CountOnline(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithTx(ctx).
		Model(&userModel{}).
		Where("username <> ?", userdomain.BuiltinRootUsername).
		Where("username <> ?", userdomain.BuiltinAdminUsername).
		Where("user_online = ?", int32(userdomain.OnlineStatusOnline)).
		Count(&count).
		Error
	return count, err
}

func (r *UserRepository) CountByRole(ctx context.Context, roleID uint64) (int64, error) {
	var count int64
	err := r.db.WithTx(ctx).
		Model(&userRoleModel{}).
		Where("role_model_id = ?", roleID).
		Count(&count).
		Error
	return count, err
}

func (r *UserRepository) CountByDeptIDs(ctx context.Context, ids []uint64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int64
	err := r.db.WithTx(ctx).
		Model(&userModel{}).
		Where("dept_id IN (?)", ids).
		Count(&count).
		Error
	return count, err
}

func (r *UserRepository) Delete(ctx context.Context, ids []uint64) error {
	return fmt.Errorf("delete users: %w", errUserRepositoryMethodNotMigrated)
}
