package controller

import (
	"context"

	"github.com/egoadmin/egoadmin/internal/app/user/service"
)

// userFindDeptNames 查找组织所属组织(层级)
func userFindDeptNames(ctx context.Context, deptSvc *service.DeptService, deptID uint64) []string {
	names := make([]string, 0)
	if deptSvc == nil || deptID == 0 {
		return names
	}
	depts, err := deptSvc.GetDeptChain(ctx, deptID)
	if err != nil {
		return names
	}
	for _, dept := range depts {
		if dept == nil {
			continue
		}
		names = append(names, dept.DeptName)
	}

	return names
}
