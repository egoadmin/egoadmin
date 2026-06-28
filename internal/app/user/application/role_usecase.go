package application

import (
	"context"
	"errors"
	"fmt"

	roledomain "github.com/egoadmin/egoadmin/internal/app/user/domain/role"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
)

// RoleUseCase orchestrates role aggregate workflows.
type RoleUseCase struct {
	role        roledomain.Repository
	mysql       mysql.MysqlInterface
	locks       RoleLocks
	assignments RoleAssignments
	permissions RolePermissionBinding
}

// RoleLocks coordinates role writes.
type RoleLocks interface {
	WithRoleCreateLock(ctx context.Context, fn func(context.Context) error) error
	WithRoleUpdateLocks(ctx context.Context, fn func(context.Context) error) error
	WithUserCreateLock(ctx context.Context, fn func(context.Context) error) error
}

// RoleAssignments queries role usage by users.
type RoleAssignments interface {
	CountByRole(ctx context.Context, roleID uint64) (int64, error)
}

// RolePermissionBinding synchronizes role API policies used by permission middleware.
type RolePermissionBinding interface {
	ReplaceRolePermissions(ctx context.Context, roleID uint64, policies []roledomain.PermissionPolicy) error
	DeleteRole(ctx context.Context, roleID uint64) error
}

// RoleOptions wires role application dependencies.
type RoleOptions struct {
	RoleRepository roledomain.Repository
	Mysql          mysql.MysqlInterface
	RoleLocks      RoleLocks
	Assignments    RoleAssignments
	Permissions    RolePermissionBinding
}

// NewRoleUseCase creates a role use case service.
func NewRoleUseCase(options RoleOptions) *RoleUseCase {
	return &RoleUseCase{
		role:        options.RoleRepository,
		mysql:       options.Mysql,
		locks:       options.RoleLocks,
		assignments: options.Assignments,
		permissions: options.Permissions,
	}
}

type SaveRoleCommand struct {
	Name        string
	Type        int32
	DataPerm    int32
	OwnerUserID uint64
	OwnerDeptID uint64
	Uses        string
	ViewMenus   string
	Desc        string
	Policies    []roledomain.PermissionPolicy
}

type CreateRoleResult struct {
	ID uint64
}

// CreateRole creates a role and synchronizes its API permission policies.
func (uc *RoleUseCase) CreateRole(ctx context.Context, cmd SaveRoleCommand) (CreateRoleResult, error) {
	var created *roledomain.Role
	err := uc.withRoleCreateLock(ctx, func(lockedCtx context.Context) error {
		if err := uc.mysql.Transaction(lockedCtx, func(txCtx context.Context) error {
			if err := uc.checkRoleNameAvailable(txCtx, cmd.Name, 0); err != nil {
				return err
			}
			created = &roledomain.Role{
				Name:        cmd.Name,
				Type:        cmd.Type,
				BuiltIn:     roledomain.NonBuiltIn,
				DataPerm:    cmd.DataPerm,
				OwnerUserID: cmd.OwnerUserID,
				OwnerDeptID: cmd.OwnerDeptID,
				Uses:        cmd.Uses,
				ViewMenus:   cmd.ViewMenus,
				Desc:        cmd.Desc,
				Policies:    roledomain.NormalizePolicies(cmd.Policies),
			}
			return uc.role.Create(txCtx, created)
		}); err != nil {
			return err
		}
		return uc.replaceRolePermissions(lockedCtx, created.ID, created.Policies)
	})
	if err != nil {
		return CreateRoleResult{}, err
	}
	return CreateRoleResult{ID: created.ID}, nil
}

// UpdateRole updates a role and synchronizes its API permission policies.
func (uc *RoleUseCase) UpdateRole(ctx context.Context, id uint64, cmd SaveRoleCommand) error {
	return uc.withRoleUpdateLocks(ctx, func(lockedCtx context.Context) error {
		updated := &roledomain.Role{
			ID:          id,
			Name:        cmd.Name,
			Type:        cmd.Type,
			DataPerm:    cmd.DataPerm,
			OwnerUserID: cmd.OwnerUserID,
			OwnerDeptID: cmd.OwnerDeptID,
			Uses:        cmd.Uses,
			ViewMenus:   cmd.ViewMenus,
			Desc:        cmd.Desc,
			Policies:    roledomain.NormalizePolicies(cmd.Policies),
		}
		if err := uc.mysql.Transaction(lockedCtx, func(txCtx context.Context) error {
			if _, err := uc.role.FindByID(txCtx, id); err != nil {
				return err
			}
			if err := uc.checkRoleNameAvailable(txCtx, cmd.Name, id); err != nil {
				return err
			}
			return uc.role.Update(txCtx, id, updated)
		}); err != nil {
			return err
		}
		return uc.replaceRolePermissions(lockedCtx, id, updated.Policies)
	})
}

// DeleteRole deletes a role when no user is assigned to it.
func (uc *RoleUseCase) DeleteRole(ctx context.Context, id uint64) error {
	return uc.withUserCreateLock(ctx, func(lockedCtx context.Context) error {
		if err := uc.mysql.Transaction(lockedCtx, func(txCtx context.Context) error {
			if err := uc.ensureRoleUnused(txCtx, id); err != nil {
				return err
			}
			return uc.role.Delete(txCtx, id)
		}); err != nil {
			return err
		}
		return uc.deleteRolePermissions(lockedCtx, id)
	})
}

// CheckDeleteRole returns a user-facing delete guard message.
func (uc *RoleUseCase) CheckDeleteRole(ctx context.Context, id uint64) (string, error) {
	var msg string
	err := uc.withUserCreateLock(ctx, func(lockedCtx context.Context) error {
		count, err := uc.roleAssignmentCount(lockedCtx, id)
		if err != nil {
			return err
		}
		if count != 0 {
			msg = roleInUseMessage(count)
		}
		return nil
	})
	return msg, err
}

func (uc *RoleUseCase) checkRoleNameAvailable(ctx context.Context, name string, currentID uint64) error {
	savedRole, err := uc.role.FindByName(ctx, name)
	if err == nil && savedRole != nil && savedRole.ID != currentID {
		return roledomain.ErrNameExists
	}
	if err != nil && !errors.Is(err, roledomain.ErrNotFound) {
		return err
	}
	return nil
}

func (uc *RoleUseCase) ensureRoleUnused(ctx context.Context, id uint64) error {
	count, err := uc.roleAssignmentCount(ctx, id)
	if err != nil {
		return err
	}
	if count != 0 {
		return roledomain.InUseError{Count: count}
	}
	return nil
}

func (uc *RoleUseCase) roleAssignmentCount(ctx context.Context, id uint64) (int64, error) {
	if uc.assignments == nil {
		return 0, nil
	}
	return uc.assignments.CountByRole(ctx, id)
}

func (uc *RoleUseCase) replaceRolePermissions(ctx context.Context, id uint64, policies []roledomain.PermissionPolicy) error {
	if uc.permissions == nil {
		return nil
	}
	return uc.permissions.ReplaceRolePermissions(ctx, id, policies)
}

func (uc *RoleUseCase) deleteRolePermissions(ctx context.Context, id uint64) error {
	if uc.permissions == nil {
		return nil
	}
	return uc.permissions.DeleteRole(ctx, id)
}

func (uc *RoleUseCase) withRoleCreateLock(ctx context.Context, fn func(context.Context) error) error {
	if uc.locks == nil {
		return fn(ctx)
	}
	return uc.locks.WithRoleCreateLock(ctx, fn)
}

func (uc *RoleUseCase) withRoleUpdateLocks(ctx context.Context, fn func(context.Context) error) error {
	if uc.locks == nil {
		return fn(ctx)
	}
	return uc.locks.WithRoleUpdateLocks(ctx, fn)
}

func (uc *RoleUseCase) withUserCreateLock(ctx context.Context, fn func(context.Context) error) error {
	if uc.locks == nil {
		return fn(ctx)
	}
	return uc.locks.WithUserCreateLock(ctx, fn)
}

func roleInUseMessage(count int64) string {
	return fmt.Sprintf("有%d个具备该角色的账号,请先编辑账号所属角色后再进行删除", count)
}
