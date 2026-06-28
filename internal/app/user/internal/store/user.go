package store

import (
	"context"
	"time"

	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/egoadmin/elib/pkg/util/xtime"
	"github.com/gotomicro/ego-component/egorm"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// 此系统将后台用户与前台用户融合在一起

const (
	// UserModelUsernameRoot 内部超级管理员
	UserModelUsernameRoot = "root"
	// UserModelUsernameAdmin 系统超级管理员
	UserModelUsernameAdmin = "admin"
)

const (
	// UserModelBuiltIn 内置用户.
	UserModelBuiltIn int32 = iota + 1
	// UserModelNonBuiltIn 普通用户.
	UserModelNonBuiltIn
)

// UserModelDefaultPasswordLen 默认密码长度
const UserModelDefaultPasswordLen = 6

const (
	// UserModelGenderHidden 隐藏
	UserModelGenderHidden = iota
	// UserModelGenderMale 男
	UserModelGenderMale
	// UserModelGenderFemale 女
	UserModelGenderFemale
)

func UserModelGenderToInt8(gender int32) (int8, bool) {
	switch gender {
	case UserModelGenderHidden:
		return UserModelGenderHidden, true
	case UserModelGenderMale:
		return UserModelGenderMale, true
	case UserModelGenderFemale:
		return UserModelGenderFemale, true
	default:
		return 0, false
	}
}

const (
	// UserModelFirstLogin 用户首次登录
	UserModelFirstLogin = iota + 1
	// UserModelNonFirstLogin 用户非首次登录
	UserModelNonFirstLogin
)

const (
	// UserModelStatusValid 用户有效
	UserModelStatusValid = iota + 1
	// UserModelStatusInvalid 用户无效
	UserModelStatusInvalid
)

const (
	// UserModelTypePlatform 平台用户
	UserModelTypePlatform = iota + 1
)

const (
	// UserModelOnline 用户在线
	UserModelOnline = iota + 1
	// UserModelOffline 用户离线
	UserModelOffline
)

// UserModel 用户.
type UserModel struct {
	xorm.Model
	BuiltIn  int32  `gorm:"int(10);not null;default:2;comment:是否内置用户,1内置用户,2普通用户"`
	Username string `gorm:"uniqueIndex;type:varchar(255);not null;comment:用户名;登录名"`
	// 采用bcrypt算法存储原始密码哈希，传输层由登录加密组件保护。
	// https://blog.csdn.net/weixin_42117918/article/details/107003197
	Password      string      `gorm:"type:varchar(255);not null;comment:用户密码;采用bcrypt存储"`
	Salt          string      `gorm:"type:varchar(255);not null;default:'';comment:用户密码盐"`
	Name          string      `gorm:"type:varchar(255);not null;default:'';comment:姓名"`
	Avatar        string      `gorm:"type:varchar(255);comment:头像"`
	Phone         *string     `gorm:"uniqueIndex;type:varchar(255);comment:手机号,中国手机不带国家代码，国际手机号格式为：国家代码-手机号"`
	Email         *string     `gorm:"uniqueIndex;type:varchar(255);comment:邮箱"`
	Realname      string      `gorm:"type:varchar(255);not null;default:'';comment:真实姓名"` // 实名认证姓名
	Nickname      string      `gorm:"type:varchar(255);not null;default:'';comment:昵称"`
	Gender        int8        `gorm:"type:tinyint(2);comment:性别,0:保密,1:男,2:女"`
	Birthday      time.Time   `gorm:"type:date;default:'1970-01-01';comment:生日"`
	FirstLogin    int32       `gorm:"type:int(10);not null;default:1;comment:是否首次登录,1:是,2:不是"`
	LastLoginIP   string      `gorm:"type:varchar(255);not null;default:'';comment:最后登录IP"`
	LastLoginAt   time.Time   `gorm:"type:datetime;default:CURRENT_TIMESTAMP;comment:最后登录时间"`
	UserStatus    int32       `gorm:"type:int(10);not null;default:1;comment:用户状态,1有效,2无效"`
	UserType      int32       `gorm:"type:int(10);not null;comment:用户类型,1:平台用户"`
	UserOnline    int32       `gorm:"type:int(10);not null;default:2;comment:用户在线状态,1:在线,2:不在线"`
	HeartbeatTime time.Time   `gorm:"type:datetime;default:CURRENT_TIMESTAMP;comment:心跳时间"`
	Remark        string      `gorm:"type:varchar(255);not null;default:'';comment:备注"`
	DeptID        uint64      `gorm:"index:idx_user_deptid,priority:1;type:bigint(20) unsigned;not null;default:0;comment:组织id"`
	Roles         []RoleModel `gorm:"many2many:user_role"` // 角色列表
}

// UserAuthSnapshot 是认证上下文校验热路径所需的最小用户字段集合.
type UserAuthSnapshot struct {
	ID         uint64
	Username   string
	UserStatus int32
	UserType   int32
	DeptID     uint64
}

// TableName 表名.
func (UserModel) TableName() string {
	return "user"
}

// SetID id设置接口.
func (m *UserModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

// BeforeCreate 创建执行前钩子函数.
func (m *UserModel) BeforeCreate(tx *gorm.DB) error {
	return mysql.SetID(m)
}

// HiddenPasswordToRPC 隐藏用户密码
func (m *UserModel) HiddenPasswordToRPC() string {
	return ""
}

// DeletedAtToRPC 最后登录时间.
func (m *UserModel) LastLoginAtToRPC() *timestamppb.Timestamp {
	return xtime.Time2Ts(m.LastLoginAt)
}

// RoleIdsToRPC 角色列表
func (m *UserModel) RoleIdsToRPC() (ids []uint64) {
	ids = make([]uint64, 0)

	if len(m.Roles) == 0 {
		return
	}

	for _, v := range m.Roles {
		ids = append(ids, v.ID)
	}

	return
}

// RoleNamesToRPC 角色名称
func (m *UserModel) RoleNamesToRPC() (names []string) {
	names = make([]string, 0)

	if len(m.Roles) == 0 {
		return
	}

	for _, v := range m.Roles {
		names = append(names, v.Name)
	}

	return
}

// UserRole 用户角色连接表
type UserRole struct {
	UserModelID uint64 `gorm:"primaryKey"`
	RoleModelID uint64 `gorm:"primaryKey"`
}

// TableName 表名.
func (UserRole) TableName() string {
	return "user_role"
}

// User 用户管理
type User struct {
	cc *egorm.Component
}

// NewUser 用户管理
func NewUser(db *egorm.Component, id xorm.IDSetter) UserInterface {
	return &User{
		cc: db,
	}
}

// Add 新增用户
//
// 如果角色不为空会创建角色关联关系
func (m *User) Add(ctx context.Context, user *UserModel) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Omit("Roles.*").Create(&user).Error

	return
}

// BatchAdd 批量新增用户
//
// 如果角色不为空会创建角色关联关系
func (m *User) BatchAdd(ctx context.Context, users []*UserModel) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Omit("Roles.*").CreateInBatches(users, 100).Error

	return
}

// Delete 删除用户
func (m *User) Delete(ctx context.Context, ids []uint64) (err error) {
	err = mysql.Transaction(ctx, m.cc, func(tx *gorm.DB) error {
		for _, id := range ids {
			if er := tx.Model(&UserModel{Model: xorm.Model{ID: id}}).Association("Roles").Clear(); er != nil {
				return er
			}

			if er := tx.Unscoped().Delete(&UserModel{}, id).Error; er != nil {
				return er
			}
		}

		return nil
	})

	return
}

// Update 修改用户
func (m *User) Update(ctx context.Context, id uint64, user *UserModel) (err error) {
	roles := user.Roles
	user.Roles = make([]RoleModel, 0)
	err = mysql.Transaction(ctx, m.cc, func(tx *gorm.DB) error {
		if er := tx.Model(&UserModel{Model: xorm.Model{ID: id}}).Association("Roles").Replace(roles); er != nil {
			return er
		}

		return tx.Omit("Roles").Model(&UserModel{Model: xorm.Model{ID: id}}).Updates(user).Error
	})

	return
}

// UpdateBase 修改用户基础信息
func (m *User) UpdateBase(ctx context.Context, id uint64, user *UserModel) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&UserModel{Model: xorm.Model{ID: id}}).Updates(user).Error

	return
}

// UpdateBaseWithoutHookAndTx 修改基础信息
//
// 不执行钩子及时间信息,无外层事务时跳过GORM默认事务
func (m *User) UpdateBaseWithoutHookAndTx(ctx context.Context, id uint64, user *UserModel) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Session(&gorm.Session{SkipDefaultTransaction: true}).
		Model(&UserModel{Model: xorm.Model{ID: id}}).
		UpdateColumns(user).Error

	return
}

// UpdateBaseWithoutHook 修改基础信息,不执行钩子及时间信息
func (m *User) UpdateBaseWithoutHook(ctx context.Context, id uint64, user *UserModel) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.
		Model(&UserModel{Model: xorm.Model{ID: id}}).
		UpdateColumns(user).Error

	return
}

// UpdatePass 修改密码
func (m *User) UpdatePass(ctx context.Context, id uint64, pass string) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.
		Model(&UserModel{Model: xorm.Model{ID: id}}).
		UpdateColumn("password", pass).Error

	return
}

// Get 查询用户
//
// 此方法会同时加载角色及权限信息
func (m *User) Get(ctx context.Context, id uint64) (user *UserModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Preload("Roles").Preload("Roles.Policies").Model(&UserModel{}).First(&user, id).Error

	return
}

// GetAuthSnapshot 查询认证上下文校验所需的用户快照
func (m *User) GetAuthSnapshot(ctx context.Context, id uint64) (snapshot *UserAuthSnapshot, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	snapshot = &UserAuthSnapshot{}
	err = db.Model(&UserModel{}).
		Select("id", "username", "user_status", "user_type", "dept_id").
		First(snapshot, id).Error

	return
}

// GetByUsername 通过用户名查找用户
//
// 此方法会同时加载角色及权限信息
func (m *User) GetByUsername(ctx context.Context, username string) (user *UserModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Preload("Roles").Preload("Roles.Policies").Where(&UserModel{Username: username}).First(&user).Error

	return
}

// GetByPhone 通过手机号查找用户
//
// 此方法会同时加载角色及权限信息
func (m *User) GetByPhone(ctx context.Context, phone string) (user *UserModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Preload("Roles").Preload("Roles.Policies").Where(&UserModel{Phone: &phone}).First(&user).Error

	return
}

// UserModelGetListOption 查询用户参数
type UserModelGetListOption struct {
	Pgopt       xorm.PaginateOption
	UserStatus  int32    // 用户状态,-1全部,1有效,2无效
	RoleID      uint64   // 角色
	Phone       string   // 电话号码
	Deptids     []uint64 // 组织
	UserOnline  int32    // 用户是否在线
	LastLoginIP string   // 用户最后登录的IP
}

// GetList 查询用户列表
func (m *User) GetList(ctx context.Context, opt UserModelGetListOption, scopes ...func(*gorm.DB) *gorm.DB) (users []*UserModel, total int64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)

	// 筛选条件
	scopes = append(scopes,
		userScopeNameHiddenRoot(),
		userScopeNameHiddenAdmin(),
		userScopeUserStatus(opt.UserStatus),
		userScopePhone(opt.Phone),
		userScopeUserOnline(opt.UserOnline),
		userScopeLastLoginIP(opt.LastLoginIP),
	)

	if len(opt.Deptids) == 1 {
		scopes = append(scopes, userScopeDept(opt.Deptids[0]))
	} else if len(opt.Deptids) > 0 {
		scopes = append(scopes, userScopeDeptsIn(opt.Deptids))
	}

	// 角色筛选
	if opt.RoleID != 0 {
		scopes = append(scopes, userScopeRoleExists(opt.RoleID))
	}

	// 分页处理
	if opt.Pgopt.Sort == "" {
		opt.Pgopt.Sort = createAt
		opt.Pgopt.Order = desc
	}

	// 查询用户数量
	err = db.Scopes(scopes...).Model(&UserModel{}).Count(&total).Error
	if err != nil {
		return
	}
	scopes = append(scopes, xorm.WithScopePaginate(opt.Pgopt)...)
	scopes = append(scopes, scopeStableIDOrder())
	err = db.Preload("Roles").Scopes(scopes...).Find(&users).Error

	return
}

// GetByIds 通过id查询用户列表
func (m *User) GetByIds(ctx context.Context, ids []uint64) (users []*UserModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Preload("Roles").Where("id IN (?)", ids).Find(&users).Error

	return
}

// GetByDeptIds 通过组织id查询用户列表
func (m *User) GetByDeptIds(ctx context.Context, ids []uint64) (users []*UserModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Where("dept_id IN (?)", ids).Find(&users).Error

	return
}

// CountByDeptIds 通过组织id统计用户
func (m *User) CountByDeptIds(ctx context.Context, ids []uint64) (count int64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&UserModel{}).Where("dept_id IN (?)", ids).Count(&count).Error

	return
}

// GetByUsernames 通过用户名查询用户
func (m *User) GetByUsernames(ctx context.Context, usernames []string) (users []*UserModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Where("username IN (?)", usernames).Find(&users).Error

	return
}

// GetByNames 通过姓名查询用户
func (m *User) GetByNames(ctx context.Context, names []string) (users []*UserModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Where("name IN (?)", names).Find(&users).Error

	return
}

// GetByPhones 通过手机号查询用户
func (m *User) GetByPhones(ctx context.Context, phones []string) (users []*UserModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Where("phone IN (?)", phones).Find(&users).Error

	return
}

// GetHeartbeatExpiredUids 获取心跳超时的用户id列表
//
// 也就是离线用户列表
func (m *User) GetHeartbeatExpiredUids(ctx context.Context, seconds int64) (uids []uint64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	beforeTime := time.Now().Add(-time.Second * time.Duration(seconds))
	err = db.Model(&UserModel{}).
		Where("heartbeat_time < ?", beforeTime).
		Where("user_online = ?", UserModelOnline).
		Pluck("id", &uids).Error

	return
}

// BatchOffline 用户批量离线
func (m *User) BatchOffline(ctx context.Context, uids []uint64) (err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&UserModel{}).Where("id IN (?)", uids).UpdateColumn("user_online", UserModelOffline).Error

	return
}

// CountOnline 统计在线用户数
func (m *User) CountOnline(ctx context.Context) (count int64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Scopes(
		userScopeNameHiddenRoot(),
		userScopeNameHiddenAdmin(),
	).
		Model(&UserModel{}).Where("user_online = ?", UserModelOnline).Count(&count).Error

	return
}

// CountByOption 统计用户
func (m *User) CountByOption(ctx context.Context, scope func(*gorm.DB) *gorm.DB) (count int64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&UserModel{}).Scopes(scope).Count(&count).Error

	return
}

// CountByRole 通过角色统计用户
func (m *User) CountByRole(ctx context.Context, roleId uint64) (count int64, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&UserRole{}).Where("role_model_id = ?", roleId).Count(&count).Error

	return
}

// GetByRoleID 通过角色查询用户.
func (m *User) GetByRoleID(ctx context.Context, roleId uint64) (users []*UserModel, err error) {
	db := mysql.DBWithContext(ctx, m.cc)
	err = db.Model(&UserModel{}).Scopes(userScopeRoleExists(roleId)).Find(&users).Error

	return
}
