package service

import (
	"context"
	"strings"
	"time"

	"github.com/egoadmin/egoadmin/internal/app/user/application"
	userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/egoadmin/elib/pkg/util/xbcrypt"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

// UserService 用户服务
type UserService struct {
	Options
}

// NewUserService 用户服务
func NewUserService(options Options) *UserService {
	return &UserService{
		Options: options,
	}
}

func (s *UserService) comparePassword(hashed string, password string) bool {
	if hashed == "" || password == "" {
		return false
	}
	return xbcrypt.Compare(hashed, password)
}

// LoginResponse 登录返回
type LoginResponse struct {
	// UserTyp 用户类型, 1:平台用户
	UserTyp int32
	// Menus 菜单列表
	Menus string
	// ExpiredAt 过期时间
	ExpiredAt time.Time
	// Token 登录token，存在过期时间，在过期前需刷新token
	Token string
	// RefreshToken 刷新token
	RefreshToken string
}

// Login 登录
// Login函数用于用户登录，接收用户名/手机号、密码、旧token和回调函数cb作为参数
func (s *UserService) Login(ctx context.Context, username string, password string, ua string, refreshToken string, loginip string, uconf *store.ConfigUser) (auth *authsession.AuthContext, resp LoginResponse, err error) {
	if s.UserUseCase != nil {
		result, er := s.UserUseCase.Login(ctx, application.LoginCommand{
			Username:     username,
			Password:     password,
			UA:           ua,
			RefreshToken: refreshToken,
			IP:           loginip,
		})
		if er != nil {
			return nil, LoginResponse{}, er
		}
		return result.Auth, LoginResponse{
			UserTyp:      result.UserType,
			Menus:        result.Menus,
			ExpiredAt:    result.ExpiresAt,
			Token:        result.AccessToken,
			RefreshToken: result.RefreshToken,
		}, nil
	}

	// TODO 用户登录失败锁定判断处理
	var (
		isRefresh bool                     // 是否是刷新登录
		issued    *authsession.IssueResult // 新token
	)
	// 如果旧token不为空，则表示是刷新登录
	if refreshToken != "" {
		isRefresh = true
		// 调用refreshLogin函数进行刷新登录
		if issued, err = s.refreshLogin(ctx, refreshToken, loginip); err != nil {
			return
		}
		username = issued.Auth.Username
	}
	// 密码登录需带上客户端设备标识信息；刷新登录沿用服务端会话里的 UA。
	if !isRefresh && ua == "" {
		err = platformi18n.ErrorFailed(ctx, "LoginInfoAbnormal", nil)

		return
	}
	// 根据用户名获取用户信息
	user, err := s.User.GetByUsername(ctx, username)
	// 如果用户名不存在，则返回用户名错误
	if err != nil && xorm.IsErrRecordNotFound(err) {
		// 根据手机号获取用户信息
		user, err = s.User.GetByPhone(ctx, username)
		if err != nil && xorm.IsErrRecordNotFound(err) {
			err = platformi18n.ErrorFailed(ctx, "LoginFailed", nil)

			return
		}
	}
	if err != nil {
		return
	}
	// 验证用户是否有效
	if user.UserStatus == store.UserModelStatusInvalid {
		err = platformi18n.ErrorFailed(ctx, "LoginFailed", nil)

		return
	}
	// 如果不是刷新登录，需要验证密码是否正确
	if !isRefresh && !s.comparePassword(user.Password, password) {
		err = platformi18n.ErrorFailed(ctx, "LoginFailed", nil)

		return
	}

	// 设置用户类型
	resp.UserTyp = store.UserModelTypePlatform

	// 如果不是刷新登录，则生成新的token
	if !isRefresh {
		// 生成token
		if issued, err = s.Auth.Issue(ctx, authsession.IssueRequest{
			UserID:   user.ID,
			Username: user.Username,
			UserType: resp.UserTyp,
			UA:       ua,
			IP:       loginip,
		}); err != nil {
			return
		}
	}
	auth = issued.Auth

	// 设置返回值
	resp.Menus = func() (menus string) {
		menuArr := make([]string, 0)
		for _, role := range user.Roles {
			menuArr = append(menuArr, strings.Split(role.ViewMenus, ",")...)
		}
		menuArr = lo.Uniq(menuArr)
		menus = strings.Join(menuArr, ",")

		return
	}()
	// 更新登录状态
	err = s.User.UpdateBaseWithoutHook(ctx, user.ID, &store.UserModel{
		UserOnline:    store.UserModelOnline,
		HeartbeatTime: time.Now(),
		LastLoginAt:   time.Now(),
		LastLoginIP:   loginip,
	})
	if err != nil {
		return
	}

	resp.ExpiredAt = issued.ExpiresAt
	resp.Token = issued.AccessToken
	resp.RefreshToken = issued.RefreshToken

	return
}

// GetMenus 获取菜单
func (s *UserService) GetMenus(ctx context.Context, id uint64) (menus string, err error) {
	user, err := s.User.Get(ctx, id)
	if err != nil {
		return
	}
	// 整理所有角色的菜单
	menuArr := make([]string, 0)
	for _, role := range user.Roles {
		menuArr = append(menuArr, strings.Split(role.ViewMenus, ",")...)
	}
	menuArr = lo.Uniq(menuArr)
	menus = strings.Join(menuArr, ",")

	return
}

// AddUser 新增用户
func (s *UserService) AddUser(ctx context.Context, user *store.UserModel) (err error) {
	scope, err := s.resolveDataScope(ctx)
	if err != nil {
		return err
	}
	if user.DeptID != 0 {
		if err = scope.EnforceDeptID(ctx, user.DeptID); err != nil {
			return err
		}
	}
	if err = s.enforceAssignableRoles(ctx, scope, user.Roles); err != nil {
		return err
	}
	// 将用户类型设置为平台用户
	user.UserType = store.UserModelTypePlatform
	if s.UserUseCase != nil {
		result, er := s.UserUseCase.CreateUser(ctx, application.CreateUserCommand{
			Username:    user.Username,
			Name:        user.Name,
			Phone:       stringValue(user.Phone),
			Gender:      userdomain.Gender(user.Gender),
			Status:      userdomain.Status(user.UserStatus),
			DeptID:      user.DeptID,
			Remark:      user.Remark,
			RoleIDs:     roleIDsFromStore(user.Roles),
			RawPassword: user.Password,
		})
		if er != nil {
			return er
		}
		user.ID = result.ID
		return nil
	}

	// 锁住新增操作
	addlock := s.UserRedis.LockAdd()
	// 尝试获取新增锁，等待时间为 5 秒
	if err = addlock.Lock(ctx, time.Second*5); err != nil {
		return
	}
	// 函数结束时释放新增锁
	defer func() {
		_ = addlock.Unlock(ctx)
	}()

	// 锁住修改操作
	lock := s.UserRedis.LockUpdate()
	// 尝试获取修改锁，等待时间为 5 秒
	if err = lock.Lock(ctx, time.Second*5); err != nil {
		return
	}
	// 函数结束时释放修改锁
	defer func() {
		_ = lock.Unlock(ctx)
	}()

	if err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
		// 对用户进行检查
		if er := s.addCheck(txCtx, user); er != nil {
			return er
		}
		defaultPassword, er := userdomain.DefaultPasswordFromPhone(stringValue(user.Phone))
		if er != nil {
			return er
		}
		hashPass, er := xbcrypt.HashAndSalt(defaultPassword)
		if er != nil {
			return er
		}
		user.Password = hashPass

		// 将用户添加到数据库中
		return s.User.Add(txCtx, user)
	}); err != nil {
		return
	}

	// 重新加载用户的 Casbin 角色
	err = s.reloadCasbinRolesForUser(ctx, user.ID)

	return
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(ctx context.Context, ids []uint64) (err error) {
	var users []*store.UserModel
	scope, err := s.resolveDataScope(ctx)
	if err != nil {
		return err
	}

	if err = s.Mysql.Transaction(ctx, func(txCtx context.Context) error {
		var er error
		// 根据用户id获取用户信息
		users, er = s.User.GetByIds(txCtx, ids)
		if er != nil {
			return er
		}
		for _, user := range users {
			if er = scope.EnforceUser(ctx, user); er != nil {
				return er
			}
		}
		// 检查是否可以删除用户
		if er = s.delCheck(ctx, users); er != nil {
			return er
		}

		// 删除用户
		return s.User.Delete(txCtx, ids)
	}); err != nil {
		return
	}
	// 删除casbin权限和token
	var firstCacheErr error
	for _, user := range users {
		// 删除用户的角色
		if er := s.Options.Casbin.DeleteRolesForUser(user.Username); er != nil {
			return er
		}
		cacheErr := deleteAuthUserSnapshotCache(ctx, s.AuthSnapshotCache(), user.ID)
		if cacheErr != nil && firstCacheErr == nil {
			firstCacheErr = cacheErr
		}
		cacheErr = deleteDataScopeCache(ctx, s.DataScopeCache(), user.ID)
		if cacheErr != nil && firstCacheErr == nil {
			firstCacheErr = cacheErr
		}
		// 删除用户的token
		if er := s.Auth.RevokeUser(ctx, user.ID, authsession.StatusRevoked); er != nil {
			return er
		}
	}

	return firstCacheErr
}

// UpdateUser 修改用户
func (s *UserService) UpdateUser(ctx context.Context, id uint64, user *store.UserModel) (err error) {
	if s.UserUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "UserUpdateUseCaseNotInitialized", nil)
	}
	scope, err := s.resolveDataScope(ctx)
	if err != nil {
		return err
	}
	savedUser, err := s.User.Get(ctx, id)
	if err != nil {
		return err
	}
	if err = scope.EnforceUser(ctx, savedUser); err != nil {
		return err
	}
	if user.DeptID != 0 {
		if err = scope.EnforceDeptID(ctx, user.DeptID); err != nil {
			return err
		}
	}
	if err = s.enforceAssignableRoles(ctx, scope, user.Roles); err != nil {
		return err
	}
	phone := ""
	if user.Phone != nil {
		phone = *user.Phone
	}
	if err = s.UserUseCase.UpdateUser(ctx, application.UpdateUserCommand{
		ID:     id,
		Name:   user.Name,
		Phone:  phone,
		Gender: userdomain.Gender(user.Gender),
		Status: userdomain.Status(user.UserStatus),
		DeptID: user.DeptID,
		RoleIDs: func() []uint64 {
			ids := make([]uint64, 0, len(user.Roles))
			for _, role := range user.Roles {
				ids = append(ids, role.ID)
			}
			return ids
		}(),
	}); err != nil {
		return err
	}
	return deleteDataScopeCache(ctx, s.DataScopeCache(), id)
}

// GetUser 查询用户
func (s *UserService) GetUser(ctx context.Context, id uint64) (user *store.UserModel, err error) {
	user, err = s.User.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	scope, err := s.resolveDataScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = scope.EnforceUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetSelfUser 查询当前用户自身资料，不接入后台数据权限。
func (s *UserService) GetSelfUser(ctx context.Context, id uint64) (user *store.UserModel, err error) {
	return s.User.Get(ctx, id)
}

// GetUserListNameOrUserName 查询用户列表 name条件[姓名/用户名]
func (s *UserService) GetUserListNameOrUserName(ctx context.Context, opt store.UserModelGetListOption, name string) (users []*store.UserModel, total int64, err error) {
	return s.getUserList(ctx, opt, store.UserScopeUsernameOrNameLike(name))
}

// GetUserListUserName 查询用户列表 name条件[用户名]
func (s *UserService) GetUserListUserName(ctx context.Context, opt store.UserModelGetListOption, name string) (users []*store.UserModel, total int64, err error) {
	return s.getUserList(ctx, opt, store.UserScopeUsernameLike(name))
}

// GetUserList 查询用户列表
func (s *UserService) getUserList(ctx context.Context, opt store.UserModelGetListOption, scopes ...func(*gorm.DB) *gorm.DB) (users []*store.UserModel, total int64, err error) {
	dataScope, err := s.resolveDataScope(ctx)
	if err != nil {
		return nil, 0, err
	}
	scopes = append(scopes, dataScope.UserScope())
	// 如果 opt.Deptids 不为空，则查询该组织及其子组织的用户
	if len(opt.Deptids) != 0 {
		ids, er := s.Dept.GetSubtreeIDs(ctx, opt.Deptids[0])
		if er != nil {
			err = er

			return
		}
		opt.Deptids = ids
	}

	// 调用 User.GetList 方法查询用户列表
	return s.User.GetList(ctx, opt, scopes...)
}

// UpdateUserPassword 修改密码
func (s *UserService) UpdateUserPassword(ctx context.Context, id uint64, oldpass, newpass string) (err error) {
	// 获取用户信息
	user, err := s.GetSelfUser(ctx, id)
	if err != nil {
		return
	}

	// 如果输入的旧密码不为空，且与数据库中的密码不一致，返回错误
	if oldpass != "" && !s.comparePassword(user.Password, oldpass) {
		err = platformi18n.ErrorFailed(ctx, "OldPasswordIncorrect", nil)
		return
	}
	// 对新密码进行哈希加盐
	hashPass, err := xbcrypt.HashAndSalt(newpass)
	if err != nil {
		return
	}
	// 更新用户密码，并删除用户的token
	if err = s.User.UpdatePass(ctx, id, hashPass); err != nil {
		return
	}
	cacheErr := deleteAuthUserSnapshotCache(ctx, s.AuthSnapshotCache(), id)
	err = s.Auth.RevokeUser(ctx, id, authsession.StatusRevoked)
	if err == nil {
		err = cacheErr
	}
	return
}

// ResetUserPassword 重置密码
func (s *UserService) ResetUserPassword(ctx context.Context, id uint64) (err error) {
	if s.UserUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "UserResetPasswordUseCaseNotInitialized", nil)
	}
	scope, err := s.resolveDataScope(ctx)
	if err != nil {
		return err
	}
	user, err := s.User.Get(ctx, id)
	if err != nil {
		return err
	}
	if err = scope.EnforceUser(ctx, user); err != nil {
		return err
	}
	if err = s.UserUseCase.ResetUserPassword(ctx, id); err != nil {
		return err
	}
	return deleteDataScopeCache(ctx, s.DataScopeCache(), id)
}

// SaveAvatar 保存用户头像公开引用ID。
func (s *UserService) SaveAvatar(ctx context.Context, uid uint64, avatarReferenceID string) (err error) {
	// 获取用户旧头像信息
	_, err = s.User.Get(ctx, uid)
	if xorm.IsErrRecordNotFound(err) {
		return platformi18n.ErrorFailed(ctx, "UserNotFound", nil)
	}
	if err != nil {
		return
	}
	// 更新用户头像信息
	err = s.User.UpdateBase(ctx, uid, &store.UserModel{
		Avatar: avatarReferenceID,
	})

	return
}

// EditCenterInfo 修改个人中心资料
// EditCenterInfo 函数用于修改用户中心信息，包括姓名、性别和电话号码(修改电话号码需验证密码)
func (s *UserService) EditCenterInfo(ctx context.Context, uid uint64, name string, gender int32, phone string, password string) (err error) {
	// 锁住修改操作
	// 创建一个 Redis 分布式锁，用于锁住修改操作
	lock := s.UserRedis.LockUpdate()
	// 尝试获取锁，等待时间为 5 秒
	if err = lock.Lock(ctx, time.Second*5); err != nil {
		return
	}
	// 函数结束时释放锁
	defer func() {
		_ = lock.Unlock(ctx)
	}()
	// 创建一个 UserModel 对象，用于存储用户信息
	genderValue, ok := store.UserModelGenderToInt8(gender)
	if !ok {
		err = platformi18n.ErrorFailed(ctx, "InvalidGender", nil)
		return
	}
	user := store.UserModel{
		Name:   name,
		Gender: genderValue,
		Phone:  &phone,
	}
	// 电话号码校验
	if phone != "" {
		// 验证密码
		us, er := s.User.Get(ctx, uid)
		if er != nil {
			err = er

			return
		}
		if !s.comparePassword(us.Password, password) {
			err = platformi18n.ErrorFailed(ctx, "LoginPasswordIncorrect", nil)

			return
		}
		// 验证唯一
		puser, er := s.User.GetByPhone(ctx, *user.Phone) // 通过手机号获取用户信息
		if er == nil && puser.ID != uid {                // 如果用户已存在且不是当前用户，则返回错误
			err = platformi18n.ErrorFailed(ctx, "PhoneExists", nil)

			return
		}
		if er != nil && !xorm.IsErrRecordNotFound(er) { // 如果未找到记录，忽略er
			err = er

			return
		}
	} else {
		user.Phone = nil
	}
	// 更新用户信息
	err = s.User.UpdateBase(ctx, uid, &user)

	return
}

// CleanLog 定时清理日志
func (s *UserService) CleanLog(ctx context.Context) (err error) {
	beforeDate := time.Now().AddDate(-2, 0, -1) // 两年前
	err = s.Log.DeleteLogBeforeDate(ctx, beforeDate)

	return
}

// OnlineUsers 读取在线用户数
func (s *UserService) OnlineUsers(ctx context.Context) (count int64, err error) {
	if s.UserUseCase == nil {
		return 0, platformi18n.ErrorFailed(ctx, "OnlineUserUseCaseNotInitialized", nil)
	}
	return s.UserUseCase.OnlineUsers(ctx)
}

// UpdateOnline 更新在线状态
func (s *UserService) UpdateOnline(ctx context.Context, username string, uid uint64) (err error) {
	if s.UserUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "OnlineUserUseCaseNotInitialized", nil)
	}
	return s.UserUseCase.MarkUserOnline(ctx, uid)
}

// MarkOfflineUsers 批量标记用户离线，仅影响在线状态展示。
func (s *UserService) MarkOfflineUsers(ctx context.Context, uids []uint64) (err error) {
	if s.UserUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "OnlineUserUseCaseNotInitialized", nil)
	}
	return s.UserUseCase.MarkUsersOffline(ctx, uids)
}

// ForceOfflineUsers 批量强制下线用户，会同时撤销登录会话。
func (s *UserService) ForceOfflineUsers(ctx context.Context, uids []uint64) (err error) {
	if s.UserUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "OnlineUserUseCaseNotInitialized", nil)
	}
	scope, err := s.resolveDataScope(ctx)
	if err != nil {
		return err
	}
	users, err := s.User.GetByIds(ctx, uids)
	if err != nil {
		return err
	}
	if len(users) != len(uids) {
		return platformi18n.ErrorFailed(ctx, "UserNotFound", nil)
	}
	for _, user := range users {
		if err = scope.EnforceUser(ctx, user); err != nil {
			return err
		}
	}
	return s.UserUseCase.ForceUsersOffline(ctx, uids)
}

func (s *UserService) enforceAssignableRoles(ctx context.Context, scope DataScope, roles []store.RoleModel) error {
	for _, role := range roles {
		savedRole, err := s.Role.Get(ctx, role.ID)
		if err != nil {
			return err
		}
		if err = scope.EnforceRole(ctx, savedRole); err != nil {
			return err
		}
	}
	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func roleIDsFromStore(roles []store.RoleModel) []uint64 {
	ids := make([]uint64, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
	}
	return ids
}

// OfflineUser 离线用户
//
// 此函数一般用于定时任务
func (s *UserService) OfflineUser(ctx context.Context) (err error) {
	if s.UserUseCase == nil {
		return platformi18n.ErrorFailed(ctx, "OnlineUserUseCaseNotInitialized", nil)
	}
	conf := s.Conf.User()
	return s.UserUseCase.OfflineExpiredUsers(ctx, application.OfflineExpiredCommand{
		Enabled:       conf.HeartbeatOfflineEnabled,
		Seconds:       conf.HeartbeatOfflineSeconds,
		RevokeSession: conf.RevokeSessionOnHeartbeatOffline,
	})
}
