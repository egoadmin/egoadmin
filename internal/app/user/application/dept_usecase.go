package application

import (
	"context"
	"errors"
	"fmt"

	deptdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/dept"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
)

// DeptUseCase orchestrates department aggregate workflows.
type DeptUseCase struct {
	dept        deptdomain.Repository
	mysql       mysql.MysqlInterface
	locks       DeptLocks
	assignments DeptAssignments
}

// DeptLocks coordinates department and user writes.
type DeptLocks interface {
	WithDeptCreateLock(ctx context.Context, fn func(context.Context) error) error
	WithDeptUpdateLocks(ctx context.Context, fn func(context.Context) error) error
	WithDeptDeleteLocks(ctx context.Context, fn func(context.Context) error) error
}

// DeptAssignments queries department usage by users.
type DeptAssignments interface {
	CountByDeptIDs(ctx context.Context, ids []uint64) (int64, error)
}

// DeptOptions wires department application dependencies.
type DeptOptions struct {
	DeptRepository deptdomain.Repository
	Mysql          mysql.MysqlInterface
	DeptLocks      DeptLocks
	Assignments    DeptAssignments
}

// NewDeptUseCase creates a department use case service.
func NewDeptUseCase(options DeptOptions) *DeptUseCase {
	return &DeptUseCase{
		dept:        options.DeptRepository,
		mysql:       options.Mysql,
		locks:       options.DeptLocks,
		assignments: options.Assignments,
	}
}

type CreateDeptCommand struct {
	ParentID uint64
	Name     string
	Leader   string
	Phone    string
	Email    string
	Remark   string
}

type CreateDeptResult struct {
	ID uint64
}

type UpdatePriorityCommand struct {
	Items []deptdomain.PriorityUpdate
}

// CreateDept creates a department under a parent department.
func (uc *DeptUseCase) CreateDept(ctx context.Context, cmd CreateDeptCommand) (CreateDeptResult, error) {
	var created *deptdomain.Dept
	err := uc.withDeptCreateLock(ctx, func(lockedCtx context.Context) error {
		return uc.mysql.Transaction(lockedCtx, func(txCtx context.Context) error {
			if err := uc.checkDeptNameAvailable(txCtx, cmd.ParentID, cmd.Name, 0); err != nil {
				return err
			}
			siblingCount, err := uc.dept.CountByParent(txCtx, cmd.ParentID)
			if err != nil {
				return err
			}
			id, err := uc.dept.NextID(txCtx)
			if err != nil {
				return err
			}
			if cmd.ParentID == deptdomain.ParentTop {
				created, err = deptdomain.NewRootChild(id, cmd.Name, siblingCount)
			} else {
				var parent *deptdomain.Dept
				parent, err = uc.dept.FindByID(txCtx, cmd.ParentID)
				if err != nil {
					return err
				}
				created, err = deptdomain.NewChild(id, parent, cmd.Name, siblingCount)
			}
			if err != nil {
				return err
			}
			created.Leader = cmd.Leader
			created.Phone = cmd.Phone
			created.Email = cmd.Email
			created.Remark = cmd.Remark
			return uc.dept.Create(txCtx, created)
		})
	})
	if err != nil {
		return CreateDeptResult{}, err
	}
	return CreateDeptResult{ID: created.ID}, nil
}

// UpdateDept updates a department name.
func (uc *DeptUseCase) UpdateDept(ctx context.Context, id uint64, name string) error {
	return uc.withDeptUpdateLocks(ctx, func(lockedCtx context.Context) error {
		return uc.mysql.Transaction(lockedCtx, func(txCtx context.Context) error {
			saved, err := uc.dept.FindByID(txCtx, id)
			if err != nil {
				return err
			}
			if err := uc.checkDeptNameAvailable(txCtx, saved.ParentID, name, id); err != nil {
				return err
			}
			return uc.dept.UpdateName(txCtx, id, name)
		})
	})
}

// UpdatePriorityDept updates sibling department priorities.
func (uc *DeptUseCase) UpdatePriorityDept(ctx context.Context, cmd UpdatePriorityCommand) error {
	if len(cmd.Items) == 0 {
		return nil
	}
	return uc.withDeptCreateLock(ctx, func(lockedCtx context.Context) error {
		return uc.mysql.Transaction(lockedCtx, func(txCtx context.Context) error {
			first, err := uc.dept.FindByID(txCtx, cmd.Items[0].ID)
			if err != nil {
				return err
			}
			count, err := uc.dept.CountByParent(txCtx, first.ParentID)
			if err != nil {
				return err
			}
			if err := deptdomain.ValidatePriorityUpdates(cmd.Items, count); err != nil {
				return err
			}
			return uc.dept.UpdatePriorities(txCtx, cmd.Items)
		})
	})
}

// DeleteDeptCascade deletes a department subtree after usage checks.
func (uc *DeptUseCase) DeleteDeptCascade(ctx context.Context, id uint64) error {
	return uc.withDeptDeleteLocks(ctx, func(lockedCtx context.Context) error {
		return uc.mysql.Transaction(lockedCtx, func(txCtx context.Context) error {
			subtree, err := uc.dept.FindSubtree(txCtx, id)
			if err != nil {
				return err
			}
			if len(subtree) == 0 {
				return deptdomain.ErrNotFound
			}
			ids := deptIDs(subtree)
			if err := uc.ensureDeptUnused(txCtx, ids); err != nil {
				return err
			}
			return uc.dept.DeleteByIDs(txCtx, ids)
		})
	})
}

// CheckDeleteDept returns a user-facing delete guard message.
func (uc *DeptUseCase) CheckDeleteDept(ctx context.Context, id uint64) (string, error) {
	subtree, err := uc.dept.FindSubtree(ctx, id)
	if err != nil {
		return "", err
	}
	if len(subtree) == 0 {
		return "", deptdomain.ErrNotFound
	}
	count, err := uc.deptAssignmentCount(ctx, deptIDs(subtree))
	if err != nil {
		return "", err
	}
	if count == 0 {
		return "", nil
	}
	return deptInUseMessage(count), nil
}

func (uc *DeptUseCase) checkDeptNameAvailable(ctx context.Context, parentID uint64, name string, currentID uint64) error {
	saved, err := uc.dept.FindByParentAndName(ctx, parentID, name)
	if err == nil && saved != nil && saved.ID != currentID {
		return deptdomain.NameExistsError{Name: name}
	}
	if err != nil && !errors.Is(err, deptdomain.ErrNotFound) {
		return err
	}
	return nil
}

func (uc *DeptUseCase) ensureDeptUnused(ctx context.Context, ids []uint64) error {
	count, err := uc.deptAssignmentCount(ctx, ids)
	if err != nil {
		return err
	}
	if count != 0 {
		return deptdomain.InUseError{Count: count}
	}
	return nil
}

func (uc *DeptUseCase) deptAssignmentCount(ctx context.Context, ids []uint64) (int64, error) {
	if uc.assignments == nil {
		return 0, nil
	}
	return uc.assignments.CountByDeptIDs(ctx, ids)
}

func (uc *DeptUseCase) withDeptCreateLock(ctx context.Context, fn func(context.Context) error) error {
	if uc.locks == nil {
		return fn(ctx)
	}
	return uc.locks.WithDeptCreateLock(ctx, fn)
}

func (uc *DeptUseCase) withDeptUpdateLocks(ctx context.Context, fn func(context.Context) error) error {
	if uc.locks == nil {
		return fn(ctx)
	}
	return uc.locks.WithDeptUpdateLocks(ctx, fn)
}

func (uc *DeptUseCase) withDeptDeleteLocks(ctx context.Context, fn func(context.Context) error) error {
	if uc.locks == nil {
		return fn(ctx)
	}
	return uc.locks.WithDeptDeleteLocks(ctx, fn)
}

func deptIDs(depts []*deptdomain.Dept) []uint64 {
	ids := make([]uint64, 0, len(depts))
	for _, dept := range depts {
		if dept != nil {
			ids = append(ids, dept.ID)
		}
	}
	return ids
}

func deptInUseMessage(count int64) string {
	return fmt.Sprintf("有%d个与该组织关联的账号, 请先编辑账号所属组织后再进行删除", count)
}
