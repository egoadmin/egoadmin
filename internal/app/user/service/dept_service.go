package service

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/app/user/application"
	deptdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/dept"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
)

// DeptService 组织服务
type DeptService struct {
	Options
}

// NewDeptService 组织服务
func NewDeptService(options Options) *DeptService {
	return &DeptService{
		Options: options,
	}
}

// AddDept 新增组织
func (s *DeptService) AddDept(ctx context.Context, dept *store.DeptModel) (err error) {
	if s.DeptUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "DeptUseCaseNotInitialized", nil)
	}
	scope, err := s.DataScope(ctx)
	if err != nil {
		return err
	}
	if err = scope.EnforceDeptMutableID(ctx, dept.ParentID); err != nil {
		return err
	}
	result, err := s.DeptUseCase.CreateDept(ctx, deptCommandFromStore(dept))
	if err != nil {
		return mapDeptDomainError(ctx, err)
	}
	dept.ID = result.ID

	return s.deleteDataScopeCacheByDeptIDs(ctx, dept.ParentID)
}

// DeleteDeptCascade 级联删除组织
// 删除组织及其子组织
func (s *DeptService) DeleteDeptCascade(ctx context.Context, id uint64) (err error) {
	if s.DeptUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "DeptUseCaseNotInitialized", nil)
	}
	scope, err := s.DataScope(ctx)
	if err != nil {
		return err
	}
	dept, err := s.Dept.GetSelf(ctx, id)
	if err != nil {
		return err
	}
	if err = scope.EnforceDeptMutableID(ctx, dept.ID); err != nil {
		return err
	}
	affectedDeptIDs, err := s.affectedDataScopeDeptIDs(ctx, id)
	if err != nil {
		return err
	}
	if err = mapDeptDomainError(ctx, s.DeptUseCase.DeleteDeptCascade(ctx, id)); err != nil {
		return err
	}
	return s.deleteDataScopeCacheForDeptIDs(ctx, affectedDeptIDs)
}

// CheckDeleteDept 检查组织是否符合删除规则
// 检查是否可以删除组织
func (s *DeptService) CheckDeleteDept(ctx context.Context, deptID uint64) (msg string, err error) {
	if s.DeptUseCase == nil {
		return "", platformi18n.ErrorFailed(ctx, "DeptUseCaseNotInitialized", nil)
	}
	scope, err := s.DataScope(ctx)
	if err != nil {
		return "", err
	}
	dept, err := s.Dept.GetSelf(ctx, deptID)
	if err != nil {
		return "", err
	}
	if err = scope.EnforceDeptMutableID(ctx, dept.ID); err != nil {
		return "", err
	}
	msg, err = s.DeptUseCase.CheckDeleteDept(ctx, deptID)
	return msg, mapDeptDomainError(ctx, err)
}

// UpdateDept 修改组织
func (s *DeptService) UpdateDept(ctx context.Context, deptID uint64, name string) (err error) {
	if s.DeptUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "DeptUseCaseNotInitialized", nil)
	}
	scope, err := s.DataScope(ctx)
	if err != nil {
		return err
	}
	dept, err := s.Dept.GetSelf(ctx, deptID)
	if err != nil {
		return err
	}
	if err = scope.EnforceDeptMutableID(ctx, dept.ID); err != nil {
		return err
	}
	if err = mapDeptDomainError(ctx, s.DeptUseCase.UpdateDept(ctx, deptID, name)); err != nil {
		return err
	}
	return s.deleteDataScopeCacheByDeptIDs(ctx, deptID)
}

// UpdatePriorityDept 修改组织顺序
func (s *DeptService) UpdatePriorityDept(ctx context.Context, depts []store.DeptModel) (err error) {
	if s.DeptUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "DeptUseCaseNotInitialized", nil)
	}
	scope, err := s.DataScope(ctx)
	if err != nil {
		return err
	}
	for _, dept := range depts {
		if err = scope.EnforceDeptMutableID(ctx, dept.ID); err != nil {
			return err
		}
	}
	if err = mapDeptDomainError(ctx, s.DeptUseCase.UpdatePriorityDept(ctx, application.UpdatePriorityCommand{
		Items: deptPriorityUpdatesFromStore(depts),
	})); err != nil {
		return err
	}
	ids := make([]uint64, 0, len(depts))
	for _, dept := range depts {
		ids = append(ids, dept.ID)
	}
	return s.deleteDataScopeCacheByDeptIDs(ctx, ids...)
}

// GetDeptByName 根据组织名称获取组织
//
// 组织名称为空时返回所有组织
// ID不为空时精确查找
func (s *DeptService) GetDeptByName(ctx context.Context, name string, id uint64) (fdepts []store.DeptModel, err error) {
	scope, err := s.DataScope(ctx)
	if err != nil {
		return nil, err
	}
	// id精确查找
	if id != 0 {
		dp, er := s.Dept.GetSelf(ctx, id)
		if er != nil {
			err = er

			return
		}
		if er = scope.EnforceDeptID(ctx, dp.ID); er != nil {
			return nil, er
		}
		pids, er := deptIDsFromCode(dp.Code)
		if er != nil {
			err = er

			return
		}
		pdps, er := s.Dept.GetByIDs(ctx, pids)
		if er != nil {
			err = er

			return
		}
		fdepts = scope.FilterDeptTree(deptAssembleTree(pdps))

		return
	}

	// 名称模糊查找
	depts, err := s.visibleDeptCandidates(ctx, scope, name)
	if err != nil {
		return
	}
	if len(depts) == 0 {
		return
	}
	ids := make(map[uint64]struct{}) // 模糊查询到的组织ids'
	treeIDs := make(map[uint64]struct{})
	for _, v := range depts {
		ids[v.ID] = struct{}{}
		subtreeIDs, er := s.Dept.GetSubtreeIDs(ctx, v.ID)
		if er != nil {
			err = er
			return
		}
		for _, id := range subtreeIDs {
			treeIDs[id] = struct{}{}
		}
		pids, er := deptIDsFromCode(v.Code)
		if er != nil {
			err = er
			return
		}
		for _, pid := range pids {
			treeIDs[pid] = struct{}{}
		}
	}
	deptIDs := make([]uint64, 0, len(treeIDs))
	for id := range treeIDs {
		deptIDs = append(deptIDs, id)
	}
	deptall, err := s.Dept.GetByIDs(ctx, deptIDs)
	if err != nil {
		return
	}
	fdepts = scope.FilterDeptTree(deptFilterByIDs(deptAssembleTree(deptall), ids))

	return
}

// GetDept 查询组织
func (s *DeptService) GetDept(ctx context.Context, id uint64) (dept *store.DeptModel, err error) {
	dept, err = s.Dept.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	scope, er := s.DataScope(ctx)
	if er != nil {
		return nil, er
	}
	if er = scope.EnforceDeptID(ctx, dept.ID); er != nil {
		return nil, er
	}

	return
}

// GetDeptAllForVisibleUsers returns all visible departments for current actor.
func (s *DeptService) GetDeptAllForVisibleUsers(ctx context.Context) (depts []*store.DeptModel, err error) {
	scope, er := s.DataScope(ctx)
	if er != nil {
		return nil, er
	}
	return s.visibleDeptCandidates(ctx, scope, "")
}

// GetTopDept 获取顶级组织
func (s *DeptService) GetTopDept(ctx context.Context) (depts []*store.DeptModel, err error) {
	scope, er := s.DataScope(ctx)
	if er != nil {
		return nil, er
	}
	return s.topVisibleDepts(ctx, scope)
}

// GetDeptChilds 获取子组织
func (s *DeptService) GetDeptChilds(ctx context.Context, parentID uint64) (depts []*store.DeptModel, err error) {
	depts, err = s.Dept.GetChilds(ctx, parentID)
	if err != nil {
		return nil, err
	}
	scope, er := s.DataScope(ctx)
	if er != nil {
		return nil, er
	}
	if er = scope.EnforceDeptID(ctx, parentID); er != nil {
		return nil, er
	}
	filtered := make([]*store.DeptModel, 0, len(depts))
	for _, dept := range depts {
		if dept != nil && scope.AllowsDeptID(dept.ID) {
			filtered = append(filtered, dept)
		}
	}
	depts = filtered

	return
}

// GetDeptChain 获取从顶层到当前组织的链路
func (s *DeptService) GetDeptChain(ctx context.Context, deptID uint64) (depts []*store.DeptModel, err error) {
	if deptID == 0 {
		return nil, nil
	}
	ids, err := s.Dept.GetAncestorIDs(ctx, deptID)
	if err != nil {
		return nil, err
	}
	ancestors, err := s.Dept.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint64]*store.DeptModel, len(ancestors))
	for _, dept := range ancestors {
		if dept == nil {
			continue
		}
		byID[dept.ID] = dept
	}
	depts = make([]*store.DeptModel, 0, len(ids))
	for _, id := range ids {
		if dept, ok := byID[id]; ok {
			depts = append(depts, dept)
		}
	}
	return depts, nil
}

func (s *DeptService) visibleDeptCandidates(ctx context.Context, scope DataScope, name string) ([]*store.DeptModel, error) {
	if scope.IsAdmin || scope.Level == DataScopeAll {
		return s.Dept.GetByName(ctx, name)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		if len(scope.DeptIDs) == 0 {
			return []*store.DeptModel{}, nil
		}
		return s.Dept.GetByIDs(ctx, scope.DeptIDs)
	}
	depts, err := s.Dept.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	filtered := make([]*store.DeptModel, 0, len(depts))
	for _, dept := range depts {
		if dept != nil && scope.AllowsDeptID(dept.ID) {
			filtered = append(filtered, dept)
		}
	}
	return filtered, nil
}

func (s *DeptService) topVisibleDepts(ctx context.Context, scope DataScope) ([]*store.DeptModel, error) {
	if scope.IsAdmin || scope.Level == DataScopeAll {
		return s.Dept.GetTopAll(ctx)
	}
	if len(scope.DeptIDs) == 0 {
		return []*store.DeptModel{}, nil
	}
	depts, err := s.Dept.GetByIDs(ctx, scope.DeptIDs)
	if err != nil {
		return nil, err
	}
	roots := visibleDeptRoots(scope, depts)
	if len(roots) == 0 {
		return []*store.DeptModel{}, nil
	}
	if scope.Level != DataScopeDeptAndSub {
		return roots, nil
	}
	return deptAssembleTreeByRoots(depts, roots), nil
}

func visibleDeptRoots(scope DataScope, depts []*store.DeptModel) []*store.DeptModel {
	if scope.IsAdmin || scope.Level == DataScopeAll {
		return depts
	}
	visible := make(map[uint64]struct{}, len(scope.DeptIDs))
	for _, id := range scope.DeptIDs {
		if id != 0 {
			visible[id] = struct{}{}
		}
	}
	roots := make([]*store.DeptModel, 0, len(depts))
	for _, dept := range depts {
		if dept == nil {
			continue
		}
		if _, ok := visible[dept.ID]; !ok {
			continue
		}
		if dept.ParentID == store.DeptModelParentTop {
			roots = append(roots, dept)
			continue
		}
		if _, ok := visible[dept.ParentID]; !ok {
			roots = append(roots, dept)
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		if roots[i].Priority == roots[j].Priority {
			return roots[i].ID < roots[j].ID
		}
		return roots[i].Priority < roots[j].Priority
	})
	return roots
}

func deptCommandFromStore(dept *store.DeptModel) application.CreateDeptCommand {
	if dept == nil {
		return application.CreateDeptCommand{}
	}
	return application.CreateDeptCommand{
		ParentID: dept.ParentID,
		Name:     dept.DeptName,
		Leader:   dept.Leader,
		Phone:    dept.Phone,
		Email:    dept.Email,
		Remark:   dept.Remark,
	}
}

func deptPriorityUpdatesFromStore(depts []store.DeptModel) []deptdomain.PriorityUpdate {
	updates := make([]deptdomain.PriorityUpdate, 0, len(depts))
	for _, dept := range depts {
		updates = append(updates, deptdomain.PriorityUpdate{
			ID:       dept.ID,
			Priority: dept.Priority,
		})
	}
	return updates
}

func mapDeptDomainError(ctx context.Context, err error) error {
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
		return userv1.ErrorUserDeptNotDel().WithMessage(deptInUseMessage(ctx, inUse.Count))
	case errors.Is(err, deptdomain.ErrInUse):
		return userv1.ErrorUserDeptNotDel().WithMessage(platformi18n.Message(ctx, "DeptInUse"))
	default:
		return err
	}
}

func deptInUseMessage(ctx context.Context, count int64) string {
	return platformi18n.Localize(ctx, "DeptInUseCount", map[string]any{"Count": count})
}

func (s *DeptService) deleteDataScopeCacheByDeptIDs(ctx context.Context, deptIDs ...uint64) error {
	affectedDeptIDs, err := s.affectedDataScopeDeptIDs(ctx, deptIDs...)
	if err != nil {
		return err
	}
	if len(affectedDeptIDs) == 0 {
		return nil
	}
	return s.deleteDataScopeCacheForDeptIDs(ctx, affectedDeptIDs)
}

func (s *DeptService) affectedDataScopeDeptIDs(ctx context.Context, deptIDs ...uint64) ([]uint64, error) {
	affectedDeptIDs := make(map[uint64]struct{}, len(deptIDs))
	for _, deptID := range deptIDs {
		if deptID == 0 {
			continue
		}
		dept, err := s.Dept.GetSelf(ctx, deptID)
		if err != nil {
			return nil, err
		}
		ids, err := deptIDsFromCode(dept.Code)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			affectedDeptIDs[id] = struct{}{}
		}
		depts, err := s.Dept.GetByCode(ctx, dept.Code)
		if err != nil {
			return nil, err
		}
		for _, dept := range depts {
			if dept != nil && dept.ID != 0 {
				affectedDeptIDs[dept.ID] = struct{}{}
			}
		}
		affectedDeptIDs[deptID] = struct{}{}
	}
	if len(affectedDeptIDs) == 0 {
		return nil, nil
	}
	ids := make([]uint64, 0, len(affectedDeptIDs))
	for id := range affectedDeptIDs {
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *DeptService) deleteDataScopeCacheForDeptIDs(ctx context.Context, ids []uint64) error {
	cache := s.DataScopeCache()
	if cache == nil {
		return nil
	}
	if len(ids) == 0 {
		return nil
	}
	users, err := s.User.GetByDeptIds(ctx, ids)
	if err != nil {
		return err
	}
	userIDs := make([]uint64, 0, len(users))
	for _, user := range users {
		if user != nil && user.ID != 0 {
			userIDs = append(userIDs, user.ID)
		}
	}
	return deleteDataScopeCache(ctx, cache, userIDs...)
}

func deptIDsFromCode(code string) ([]uint64, error) {
	if code == "" {
		return nil, nil
	}
	parts := strings.Split(code, "-")
	ids := make([]uint64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
