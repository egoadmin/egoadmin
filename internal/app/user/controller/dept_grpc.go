package controller

import (
	"context"
	"errors"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/user/application"
	deptdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/dept"
	"github.com/egoadmin/egoadmin/internal/app/user/internal/auditlog"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/app/user/service"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/jinzhu/copier"
)

// DeptGrpc 组织grpc
type DeptGRPC struct {
	dept        *service.DeptService
	deptUseCase *application.DeptUseCase
	logger      auditlog.Loger
}

// NewDeptGRPCController 实例化组织grpc
func NewDeptGRPCController(dept *service.DeptService, deptUseCase *application.DeptUseCase, logger auditlog.Loger) *DeptGRPC {
	return &DeptGRPC{
		dept:        dept,
		deptUseCase: deptUseCase,
		logger:      logger,
	}
}

// AddDept 新增组织
func (s *DeptGRPC) AddDept(ctx context.Context, in *userv1.AddDeptRequest) (out *userv1.AddDeptResponse, err error) {
	out = &userv1.AddDeptResponse{}

	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户管理-组织管理", "新增", "新增组织", in)
		}
	}()

	dept := &store.DeptModel{
		ParentID: in.GetParentId(),
		DeptName: in.GetDept().GetDeptName(),
		Leader:   in.GetDept().GetLeader(),
		Phone:    in.GetDept().GetPhone(),
		Email:    in.GetDept().GetEmail(),
		Remark:   in.GetDept().GetRemark(),
	}
	err = s.dept.AddDept(ctx, dept)
	if err = mapDeptError(ctx, err); err != nil {
		return
	}
	out.Id = dept.ID

	return
}

// DeleteDeptCascade 级联删除组织(含子组织)
func (s *DeptGRPC) DeleteDeptCascade(ctx context.Context, in *userv1.DeleteDeptCascadeRequest) (out *userv1.DeleteDeptCascadeResponse, err error) {
	out = &userv1.DeleteDeptCascadeResponse{}

	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户管理-组织管理", "删除", "删除组织", in)
		}
	}()

	err = mapDeptError(ctx, s.dept.DeleteDeptCascade(ctx, in.GetId()))

	return
}

// UpdateDept 修改组织
func (s *DeptGRPC) UpdateDept(ctx context.Context, in *userv1.UpdateDeptRequest) (out *userv1.UpdateDeptResponse, err error) {
	out = &userv1.UpdateDeptResponse{}

	defer func() {
		if err == nil {
			s.logger.Save(ctx, "用户管理-组织管理", "编辑", "编辑组织", in)
		}
	}()

	err = mapDeptError(ctx, s.dept.UpdateDept(ctx, in.GetId(), in.GetDept().GetDeptName()))

	return
}

// UpdatePriorityDept 修改排序
//
// 修改时只允许修改同层级的排序,在前端将同级的所有数据按顺序排完后一次修改
func (s *DeptGRPC) UpdatePriorityDept(ctx context.Context, in *userv1.UpdatePriorityDeptRequest) (out *userv1.UpdatePriorityDeptResponse, err error) {
	out = &userv1.UpdatePriorityDeptResponse{}

	if len(in.GetPriorities()) == 0 {
		return
	}

	err = mapDeptError(ctx, s.dept.UpdatePriorityDept(ctx, deptPriorityUpdates(in.GetPriorities())))

	return
}

func deptPriorityUpdates(in []*userv1.UpdatePriorityDeptItem) []store.DeptModel {
	updates := make([]store.DeptModel, 0, len(in))
	for _, item := range in {
		updates = append(updates, store.DeptModel{
			Model:    xorm.Model{ID: item.GetId()},
			Priority: item.GetPriority(),
		})
	}
	return updates
}

// GetDeptByName 根据组织名称获取组织
//
// 组织名称为空时返回所有组织
// ID不为空时精确查找
func (s *DeptGRPC) GetDeptByName(ctx context.Context, in *userv1.GetDeptByNameRequest) (out *userv1.GetDeptByNameResponse, err error) {
	out = &userv1.GetDeptByNameResponse{Depts: []*userv1.Dept{}}
	fdepts, err := s.dept.GetDeptByName(ctx, in.GetName(), in.GetId())
	if err != nil {
		return
	}
	err = copier.Copy(&out.Depts, &fdepts)

	return
}

// GetDept 查询组织
func (s *DeptGRPC) GetDept(ctx context.Context, in *userv1.GetDeptRequest) (out *userv1.GetDeptResponse, err error) {
	out = &userv1.GetDeptResponse{
		Dept: &userv1.Dept{},
	}

	dept, err := s.dept.GetDept(ctx, in.GetId())
	if err != nil {
		return
	}

	err = copier.Copy(&out.Dept, &dept)

	return
}

// GetDeptTop 获取顶级组织
func (s *DeptGRPC) GetDeptTop(ctx context.Context, in *userv1.GetDeptTopRequest) (out *userv1.GetDeptTopResponse, err error) {
	out = &userv1.GetDeptTopResponse{
		Depts: make([]*userv1.Dept, 0),
	}

	depts, err := s.dept.GetTopDept(ctx)
	if err != nil {
		return
	}

	err = copier.Copy(&out.Depts, &depts)

	return
}

// GetDeptChild 获取子组织
func (s *DeptGRPC) GetDeptChild(ctx context.Context, in *userv1.GetDeptChildRequest) (out *userv1.GetDeptChildResponse, err error) {
	out = &userv1.GetDeptChildResponse{
		Depts: make([]*userv1.Dept, 0),
	}

	depts, err := s.dept.GetDeptChilds(ctx, in.GetId())
	if err != nil {
		return
	}

	err = copier.Copy(&out.Depts, &depts)

	return
}

// CheckDeptDelete 检查组织是否可删除
func (s *DeptGRPC) CheckDeptDelete(ctx context.Context, in *userv1.CheckDeptDeleteRequest) (out *userv1.CheckDeptDeleteResponse, err error) {
	out = &userv1.CheckDeptDeleteResponse{}

	rejectMsg, err := s.dept.CheckDeleteDept(ctx, in.GetId())
	if err != nil {
		err = mapDeptError(ctx, err)
		return
	}
	if rejectMsg == "" {
		out.IsAllow = true

		return
	}
	out.IsAllow = false
	out.Msg = rejectMsg

	return
}

func mapDeptError(ctx context.Context, err error) error {
	var nameExists deptdomain.NameExistsError
	var maxLevel deptdomain.MaxLevelError
	var inUse deptdomain.InUseError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &nameExists):
		return platformi18n.ErrorFailed(ctx, "DeptNameExistsWithName", map[string]any{"Name": nameExists.Name})
	case errors.Is(err, deptdomain.ErrNameExists):
		return platformi18n.ErrorFailed(ctx, "DeptNameExists", nil)
	case errors.As(err, &maxLevel):
		return platformi18n.ErrorFailed(ctx, "DeptMaxLevelExceeded", map[string]any{"MaxLevel": maxLevel.MaxLevel})
	case errors.Is(err, deptdomain.ErrTooManySiblings):
		return platformi18n.ErrorFailed(ctx, "DeptTooManySiblings", nil)
	case errors.Is(err, deptdomain.ErrPriorityChanged):
		return platformi18n.ErrorFailed(ctx, "DataChangedRetry", nil)
	case errors.Is(err, deptdomain.ErrInvalidPriority):
		return platformi18n.ErrorFailed(ctx, "InvalidPriority", nil)
	case errors.Is(err, deptdomain.ErrNotFound):
		return platformi18n.ErrorFailed(ctx, "DeptNotFound", nil)
	case errors.As(err, &inUse):
		return userv1.ErrorUserDeptNotDel().WithMessage(deptDeleteInUseMessage(ctx, inUse.Count))
	case errors.Is(err, deptdomain.ErrInUse):
		return userv1.ErrorUserDeptNotDel().WithMessage(platformi18n.Message(ctx, "DeptInUse"))
	default:
		return err
	}
}

func deptDeleteInUseMessage(ctx context.Context, count int64) string {
	return platformi18n.Localize(ctx, "DeptInUseCount", map[string]any{"Count": count})
}
