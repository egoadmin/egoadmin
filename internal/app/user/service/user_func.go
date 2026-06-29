package service

import (
	"context"
	"strconv"

	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/egoadmin/elib/pkg/util/xbcrypt"
	"github.com/egoadmin/elib/pkg/util/xorm"
)

// CreateSuperuser 创建超级管理员
//
// 本系统包含root及admin账户
// 创建超级用户
func (s *UserService) CreateSuperuser(ctx context.Context) (err error) {
	err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
		var (
			addRoot  bool // 是否添加root用户
			addAdmin bool // 是否添加admin用户
		)

		// 检查是否已存在root用户
		_, er := s.User.GetByUsername(txCtx, store.UserModelUsernameRoot)
		if er != nil && !xorm.IsErrRecordNotFound(er) {
			return er
		}
		if xorm.IsErrRecordNotFound(er) {
			addRoot = true
		}

		// 检查是否已存在admin用户
		_, er = s.User.GetByUsername(txCtx, store.UserModelUsernameAdmin)
		if er != nil && !xorm.IsErrRecordNotFound(er) {
			return er
		}
		if xorm.IsErrRecordNotFound(er) {
			addAdmin = true
		}

		// 获取配置文件中的root和admin密码，并进行加密
		conf := s.Options.Conf.User()
		rootPassword, er := xbcrypt.HashAndSalt(conf.RootPassword)
		if er != nil {
			return er
		}
		adminPassword, er := xbcrypt.HashAndSalt(conf.AdminPassword)
		if er != nil {
			return er
		}

		// 创建root用户
		root := store.UserModel{
			BuiltIn:  store.UserModelBuiltIn,
			Username: store.UserModelUsernameRoot,
			Password: rootPassword,
			Name:     store.UserModelUsernameRoot,
			Gender:   0,
			UserType: store.UserModelTypePlatform,
		}

		// 创建admin用户
		admin := store.UserModel{
			BuiltIn:  store.UserModelBuiltIn,
			Username: store.UserModelUsernameAdmin,
			Password: adminPassword,
			Name:     store.UserModelUsernameAdmin,
			Gender:   0,
			UserType: store.UserModelTypePlatform,
		}

		// 添加root用户
		if addRoot && conf.RootPassword != "" {
			if er = s.User.Add(txCtx, &root); er != nil {
				return er
			}
		}

		// 添加admin用户
		if addAdmin && conf.AdminPassword != "" {
			if er = s.User.Add(txCtx, &admin); er != nil {
				return er
			}
		}

		return nil
	})
	return
}

// refreshLogin 刷新登录
func (s *UserService) refreshLogin(ctx context.Context, refreshToken string, loginip string) (*authsession.IssueResult, error) {
	return s.Auth.Refresh(ctx, authsession.RefreshRequest{
		RefreshToken: refreshToken,
		IP:           loginip,
	})
}

// addCheck 新增用户检查
func (s *UserService) addCheck(ctx context.Context, user *store.UserModel) (err error) {
	// 用户名不能是内置超级管理员
	if user.Username == store.UserModelUsernameRoot || user.Username == store.UserModelUsernameAdmin {
		err = platformi18n.ErrorFailed(ctx, "UsernameNotAllowed", nil) // 如果用户名是内置超级管理员或管理员，则返回错误

		return
	}
	_, err = s.User.GetByUsername(ctx, user.Username) // 通过用户名获取用户信息
	if err == nil {
		err = platformi18n.ErrorFailed(ctx, "UsernameExists", nil) // 如果用户已存在，则返回错误

		return
	}
	if err != nil && !xorm.IsErrRecordNotFound(err) { // 如果不是未找到记录的错误，则返回错误
		return
	}
	if user.Phone == nil { // 如果用户手机号为空，则返回nil
		err = nil

		return
	}
	_, err = s.User.GetByPhone(ctx, *user.Phone) // 通过手机号获取用户信息
	if err == nil {
		err = platformi18n.ErrorFailed(ctx, "PhoneExists", nil) // 如果手机号已被使用，则返回错误

		return
	}
	if xorm.IsErrRecordNotFound(err) { // 如果未找到记录，则返回nil
		err = nil

		return
	}

	return
}

// delCheck 删除检查
func (s *UserService) delCheck(ctx context.Context, users []*store.UserModel) (err error) {
	if len(users) == 0 { // 如果用户列表为空，则返回nil
		return
	}
	for _, user := range users { // 遍历用户列表
		// 不允许删除的用户
		if user.BuiltIn == store.UserModelBuiltIn { // 如果用户是内置用户，则返回错误
			err = platformi18n.ErrorFailed(ctx, "CannotDeleteBuiltinUser", nil)

			return
		}
	}

	return
}

// updateCheck 修改检查
//
//nolint:unused // 预留:用户更新前的业务校验
func (s *UserService) updateCheck(ctx context.Context, id uint64, user *store.UserModel) (err error) {
	// 用户名不能是内置超级管理员
	if user.Username == store.UserModelUsernameRoot || user.Username == store.UserModelUsernameAdmin {
		err = platformi18n.ErrorFailed(ctx, "UsernameNotAllowed", nil) // 如果用户名是内置超级管理员或管理员，则返回错误

		return
	}
	// 新用户名不能与他之外的用户名一样
	if user.Username != "" { // 如果新用户名不为空
		olduser, er := s.User.GetByUsername(ctx, user.Username) // 通过新用户名获取用户信息
		if er == nil && olduser.ID != id {                      // 如果用户已存在且不是当前用户，则返回错误
			err = platformi18n.ErrorFailed(ctx, "UsernameExists", nil)

			return
		}
		if er != nil && !xorm.IsErrRecordNotFound(er) { // 如果不是未找到记录的错误，则返回错误
			return
		}
		if user.Phone == nil { // 如果用户手机号为空，则返回nil
			err = nil

			return
		}
	}
	puser, err := s.User.GetByPhone(ctx, *user.Phone) // 通过手机号获取用户信息
	if err == nil && puser.ID != id {                 // 如果用户已存在且不是当前用户，则返回错误
		err = platformi18n.ErrorFailed(ctx, "PhoneExists", nil)

		return
	}
	if xorm.IsErrRecordNotFound(err) { // 如果未找到记录，则返回nil
		err = nil

		return
	}

	return
}

// reloadCasbinRolesForUser 重载用户casbin权限
func (s *UserService) reloadCasbinRolesForUser(ctx context.Context, id uint64) (err error) {
	savedUser, err := s.User.Get(ctx, id) // 通过用户ID获取用户信息
	if err != nil {
		return
	}
	roleNames := make([]string, 0, len(savedUser.Roles)) // 创建角色名称列表
	for _, role := range savedUser.Roles {               // 遍历用户角色列表
		name := strconv.FormatUint(role.ID, 10) // 将角色ID转换为字符串
		roleNames = append(roleNames, name)     // 将角色名称添加到角色名称列表中
	}
	oldroles, err := s.Casbin.Enforcer().GetRolesForUser(savedUser.Username) // 获取用户角色变更前的所有角色
	if err != nil {
		return
	}
	// oldroles - roleNames 求差集,得到需要排除的角色
	for i := 0; i < len(oldroles); i++ {
		count := 0
		for j := 0; j < len(roleNames); j++ {
			if oldroles[i] == roleNames[j] {
				break
			}
			count++
		}
		if count == len(roleNames) {
			_, _ = s.Casbin.Enforcer().DeleteRoleForUser(savedUser.Username, oldroles[i]) // 移除角色
		}
	}
	err = s.Casbin.AddRolesForUser(savedUser.Username, roleNames) // 为用户添加角色

	return
}

// reloadCasbinRolesForUsers 重载多个用户casbin权限
// func (s *UserService) reloadCasbinRolesForUsers(ctx context.Context, roleid uint64, usernames []string) (err error) {
// 	rolename := strconv.FormatUint(roleid, 10) // 将角色ID转换为字符串
// 	policies := [][]string{}                   // 创建策略列表
// 	for _, name := range usernames {           // 遍历用户名列表
// 		policies = append(policies, []string{
// 			name, rolename, // 将用户名和角色名称添加到策略列表中
// 		})
// 	}

// 	ok, err := s.Options.Casbin.Enforcer().AddGroupingPolicies(policies) // 为用户添加角色
// 	if err != nil {
// 		return
// 	}
// 	if !ok {
// 		err = ecode.ErrorFailed().WithMessage("角色更新失败") // 如果更新失败，则返回错误
// 	}

// 	return
// }
