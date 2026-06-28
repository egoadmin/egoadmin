package store

import (
	"context"

	"gorm.io/gorm"
)

// UserInterface 用户管理
type UserInterface interface {
	// Add 新增用户
	//
	// 如果角色不为空会创建角色关联关系
	Add(ctx context.Context, user *UserModel) (err error)
	// BatchAdd 批量新增用户
	//
	// 如果角色不为空会创建角色关联关系
	BatchAdd(ctx context.Context, users []*UserModel) (err error)
	// Delete 删除用户
	//
	// 会同时删除与角色的关联
	Delete(ctx context.Context, ids []uint64) (err error)
	// Update 修改用户
	//
	// 会同时修改与角色的关联
	Update(ctx context.Context, id uint64, user *UserModel) (err error)
	// UpdateBase 修改用户基础信息
	UpdateBase(ctx context.Context, id uint64, user *UserModel) (err error)
	// UpdateBaseWithoutHook 修改基础信息,不执行钩子及时间信息
	UpdateBaseWithoutHook(ctx context.Context, id uint64, user *UserModel) (err error)
	// UpdateBaseWithoutHookAndTx 修改基础信息
	//
	// 不执行钩子及时间信息,无外层事务时跳过GORM默认事务
	UpdateBaseWithoutHookAndTx(ctx context.Context, id uint64, user *UserModel) (err error)
	// UpdatePass 修改密码
	UpdatePass(ctx context.Context, id uint64, pass string) (err error)
	// Get 查询用户
	//
	// 此方法会同时加载角色及权限信息
	Get(ctx context.Context, id uint64) (user *UserModel, err error)
	// GetAuthSnapshot 查询认证上下文校验所需的用户快照
	//
	// 此方法只查询鉴权热路径需要的基础字段，不加载角色和权限。
	GetAuthSnapshot(ctx context.Context, id uint64) (snapshot *UserAuthSnapshot, err error)
	// GetByUsername 通过用户名查找用户
	//
	// 此方法会同时加载角色及权限信息
	GetByUsername(ctx context.Context, username string) (user *UserModel, err error)
	// GetByPhone 通过手机号查找用户
	//
	// 此方法会同时加载角色及权限信息
	GetByPhone(ctx context.Context, phone string) (user *UserModel, err error)
	// GetList 查询用户列表
	GetList(ctx context.Context, opt UserModelGetListOption, scopes ...func(*gorm.DB) *gorm.DB) (users []*UserModel, total int64, err error)
	// GetByIds 通过id查询用户列表
	//
	// 此方法会同时加载角色
	GetByIds(ctx context.Context, ids []uint64) (users []*UserModel, err error)
	// GetByDeptIds 通过组织id查询用户列表
	GetByDeptIds(ctx context.Context, ids []uint64) (users []*UserModel, err error)
	// CountByDeptIds 通过组织id统计用户
	CountByDeptIds(ctx context.Context, ids []uint64) (count int64, err error)
	// GetByUsernames 通过用户名查询用户
	GetByUsernames(ctx context.Context, usernames []string) (users []*UserModel, err error)
	// GetByNames 通过姓名查询用户
	GetByNames(ctx context.Context, names []string) (users []*UserModel, err error)
	// GetByPhones 通过手机号查询用户
	GetByPhones(ctx context.Context, phones []string) (users []*UserModel, err error)
	// GetHeartbeatExpiredUids 获取心跳超时的用户id列表
	//
	// 也就是离线用户列表
	GetHeartbeatExpiredUids(ctx context.Context, seconds int64) (uids []uint64, err error)
	// BatchOffline 用户批量离线
	BatchOffline(ctx context.Context, uids []uint64) (err error)
	// CountOnline 统计在线用户数
	CountOnline(ctx context.Context) (count int64, err error)
	// CountByOption 统计用户
	CountByOption(ctx context.Context, scope func(*gorm.DB) *gorm.DB) (count int64, err error)
	// CountByRole 通过角色统计用户
	CountByRole(ctx context.Context, roleId uint64) (count int64, err error)
	// GetByRoleID 通过角色查询用户
	GetByRoleID(ctx context.Context, roleId uint64) (users []*UserModel, err error)
}
