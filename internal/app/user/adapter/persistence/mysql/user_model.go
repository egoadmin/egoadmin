package mysql

import (
	"time"

	userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

type userModel struct {
	xorm.Model
	Username      string
	Password      string
	Name          string
	Phone         *string
	Gender        int8
	UserStatus    int32
	UserType      int32
	UserOnline    int32
	Remark        string
	DeptID        uint64
	Roles         []roleModel `gorm:"many2many:user_role"`
	HeartbeatTime time.Time
}

func (userModel) TableName() string {
	return "user"
}

func (m *userModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

func (m *userModel) BeforeCreate(tx *gorm.DB) error {
	return platformmysql.SetID(m)
}

type roleModel struct {
	xorm.Model
	ViewMenus string
}

func (roleModel) TableName() string {
	return "role"
}

type userRoleModel struct {
	UserModelID uint64 `gorm:"primaryKey"`
	RoleModelID uint64 `gorm:"primaryKey"`
}

func (userRoleModel) TableName() string {
	return "user_role"
}

func (m *userModel) toDomain() *userdomain.User {
	if m == nil {
		return nil
	}
	phone := ""
	if m.Phone != nil {
		phone = *m.Phone
	}
	return &userdomain.User{
		ID:            m.ID,
		Username:      m.Username,
		PasswordHash:  m.Password,
		Name:          m.Name,
		Phone:         phone,
		Gender:        userdomain.Gender(m.Gender),
		Status:        userdomain.Status(m.UserStatus),
		Type:          m.UserType,
		OnlineStatus:  userdomain.OnlineStatus(m.UserOnline),
		DeptID:        m.DeptID,
		Remark:        m.Remark,
		RoleIDs:       roleIDsFromModels(m.Roles),
		RoleMenus:     roleMenusFromModels(m.Roles),
		HeartbeatTime: m.HeartbeatTime,
	}
}

func userModelFromDomain(u *userdomain.User) *userModel {
	if u == nil {
		return nil
	}
	phone := u.Phone
	return &userModel{
		Model:         xorm.Model{ID: u.ID},
		Username:      u.Username,
		Password:      u.PasswordHash,
		Name:          u.Name,
		Phone:         &phone,
		Gender:        int8(u.Gender),
		UserStatus:    int32(u.Status),
		UserType:      u.Type,
		UserOnline:    int32(u.OnlineStatus),
		DeptID:        u.DeptID,
		Remark:        u.Remark,
		Roles:         roleModelsFromIDs(u.RoleIDs),
		HeartbeatTime: u.HeartbeatTime,
	}
}

func userUpdateModelFromDomain(u *userdomain.User) *userModel {
	if u == nil {
		return nil
	}
	phone := u.Phone
	return &userModel{
		Name:       u.Name,
		Phone:      &phone,
		Gender:     int8(u.Gender),
		UserStatus: int32(u.Status),
		DeptID:     u.DeptID,
		Roles:      roleModelsFromIDs(u.RoleIDs),
	}
}

func roleModelsFromIDs(ids []uint64) []roleModel {
	roles := make([]roleModel, 0, len(ids))
	for _, id := range ids {
		roles = append(roles, roleModel{Model: xorm.Model{ID: id}})
	}
	return roles
}

func roleIDsFromModels(roles []roleModel) []uint64 {
	ids := make([]uint64, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
	}
	return ids
}

func roleMenusFromModels(roles []roleModel) []string {
	menus := make([]string, 0, len(roles))
	for _, role := range roles {
		menus = append(menus, role.ViewMenus)
	}
	return menus
}
