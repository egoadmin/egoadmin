package mysql

import (
	deptdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/dept"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

type deptModel struct {
	xorm.Model
	Code     string
	ParentID uint64
	DeptName string
	Leader   string
	Phone    string
	Email    string
	Remark   string
	Priority int32
	Status   int32
	Level    int32
}

func (deptModel) TableName() string {
	return "dept"
}

func (m *deptModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

func (m *deptModel) BeforeCreate(tx *gorm.DB) error {
	return platformmysql.SetID(m)
}

func deptModelFromDomain(dept *deptdomain.Dept) *deptModel {
	if dept == nil {
		return nil
	}
	return &deptModel{
		Model:    xorm.Model{ID: dept.ID},
		Code:     dept.Code,
		ParentID: dept.ParentID,
		DeptName: dept.Name,
		Leader:   dept.Leader,
		Phone:    dept.Phone,
		Email:    dept.Email,
		Remark:   dept.Remark,
		Priority: dept.Priority,
		Status:   dept.Status,
		Level:    dept.Level,
	}
}

func (m *deptModel) toDomain() *deptdomain.Dept {
	if m == nil {
		return nil
	}
	return &deptdomain.Dept{
		ID:       m.ID,
		Code:     m.Code,
		ParentID: m.ParentID,
		Name:     m.DeptName,
		Leader:   m.Leader,
		Phone:    m.Phone,
		Email:    m.Email,
		Remark:   m.Remark,
		Priority: m.Priority,
		Status:   m.Status,
		Level:    m.Level,
	}
}

func deptModelsToDomain(models []deptModel) []*deptdomain.Dept {
	depts := make([]*deptdomain.Dept, 0, len(models))
	for i := range models {
		depts = append(depts, models[i].toDomain())
	}
	return depts
}
