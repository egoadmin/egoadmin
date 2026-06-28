package application

import (
	"context"
	"errors"
	"time"

	userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xbcrypt"
)

// UserUseCase orchestrates user aggregate workflows.
type UserUseCase struct {
	user     userdomain.Repository
	mysql    mysql.MysqlInterface
	auth     authsession.Interface
	snapshot AuthSnapshotCache
	locks    UserLocks
	roles    RoleBinding
}

// AuthSnapshotCache removes cached auth snapshots after identity-sensitive changes.
type AuthSnapshotCache interface {
	DeleteUser(ctx context.Context, userID uint64) error
}

// UserLocks coordinates writes that must not run concurrently.
type UserLocks interface {
	WithCreateLocks(ctx context.Context, fn func(context.Context) error) error
	WithUpdateLock(ctx context.Context, fn func(context.Context) error) error
}

// RoleBinding synchronizes role assignments used by permission middleware.
type RoleBinding interface {
	ReplaceUserRoles(ctx context.Context, username string, roleIDs []uint64) error
}

// UserOptions wires user application dependencies.
type UserOptions struct {
	UserRepository userdomain.Repository
	Mysql          mysql.MysqlInterface
	Auth           authsession.Interface
	AuthSnapshot   AuthSnapshotCache
	UserLocks      UserLocks
	RoleBinding    RoleBinding
}

// NewUserUseCase creates a user use case service.
func NewUserUseCase(options UserOptions) *UserUseCase {
	return &UserUseCase{
		user:     options.UserRepository,
		mysql:    options.Mysql,
		auth:     options.Auth,
		snapshot: options.AuthSnapshot,
		locks:    options.UserLocks,
		roles:    options.RoleBinding,
	}
}

// CreateUserCommand carries create-user input from transport adapters.
type CreateUserCommand struct {
	Username    string
	Name        string
	Phone       string
	Gender      userdomain.Gender
	Status      userdomain.Status
	DeptID      uint64
	Remark      string
	RoleIDs     []uint64
	RawPassword string
}

// CreateUserResult carries create-user output for transport adapters.
type CreateUserResult struct {
	ID uint64
}

// UpdateUserCommand carries update-user input from transport adapters.
type UpdateUserCommand struct {
	ID      uint64
	Name    string
	Phone   string
	Gender  userdomain.Gender
	Status  userdomain.Status
	DeptID  uint64
	RoleIDs []uint64
}

// LoginCommand carries password-login or refresh-login input.
type LoginCommand struct {
	Username     string
	Password     string
	UA           string
	RefreshToken string
	IP           string
}

// LoginResult carries login output for transport adapters.
type LoginResult struct {
	Auth         *authsession.AuthContext
	UserType     int32
	Menus        string
	ExpiresAt    time.Time
	AccessToken  string
	RefreshToken string
}

// OfflineExpiredCommand carries heartbeat-expiry policy from runtime config.
type OfflineExpiredCommand struct {
	Enabled       bool
	Seconds       int64
	RevokeSession bool
}

// CreateUser creates a platform user and synchronizes permission role bindings.
func (uc *UserUseCase) CreateUser(ctx context.Context, cmd CreateUserCommand) (CreateUserResult, error) {
	if cmd.RawPassword != "" {
		return CreateUserResult{}, ErrSubmittedPassword
	}
	var result CreateUserResult
	err := uc.withCreateLocks(ctx, func(lockedCtx context.Context) error {
		created := &userdomain.User{}
		if err := uc.mysql.Transaction(lockedCtx, func(txCtx context.Context) error {
			if err := uc.checkCreateUser(txCtx, cmd); err != nil {
				return err
			}
			defaultPassword, err := userdomain.DefaultPasswordFromPhone(cmd.Phone)
			if err != nil {
				return err
			}
			hashPass, err := xbcrypt.HashAndSalt(defaultPassword)
			if err != nil {
				return err
			}
			now := time.Now()
			created = &userdomain.User{
				Username:      cmd.Username,
				PasswordHash:  hashPass,
				Name:          cmd.Name,
				Phone:         cmd.Phone,
				Gender:        cmd.Gender,
				Status:        cmd.Status,
				Type:          userdomain.TypePlatform,
				OnlineStatus:  userdomain.OnlineStatusOffline,
				DeptID:        cmd.DeptID,
				Remark:        cmd.Remark,
				RoleIDs:       append([]uint64(nil), cmd.RoleIDs...),
				HeartbeatTime: now,
			}
			return uc.user.Create(txCtx, created)
		}); err != nil {
			return err
		}

		if err := replaceUserRoles(lockedCtx, uc.roles, created.Username, created.RoleIDs); err != nil {
			return err
		}
		result.ID = created.ID
		return nil
	})
	return result, err
}

// UpdateUser updates user profile, role bindings, auth cache, and sessions.
func (uc *UserUseCase) UpdateUser(ctx context.Context, cmd UpdateUserCommand) error {
	return uc.withUpdateLock(ctx, func(lockedCtx context.Context) error {
		var oldUser *userdomain.User
		if err := uc.mysql.Transaction(lockedCtx, func(txCtx context.Context) error {
			var err error
			oldUser, err = uc.user.FindByID(txCtx, cmd.ID)
			if err != nil {
				return err
			}
			if err = uc.checkUpdateUser(txCtx, cmd); err != nil {
				return err
			}
			return uc.user.Update(txCtx, cmd.ID, &userdomain.User{
				Name:    cmd.Name,
				Phone:   cmd.Phone,
				Gender:  cmd.Gender,
				Status:  cmd.Status,
				DeptID:  cmd.DeptID,
				RoleIDs: append([]uint64(nil), cmd.RoleIDs...),
			})
		}); err != nil {
			return err
		}

		cacheErr := deleteAuthSnapshot(lockedCtx, uc.snapshot, cmd.ID)
		if oldUser.Status != cmd.Status && cmd.Status == userdomain.StatusInvalid {
			if err := uc.auth.RevokeUser(lockedCtx, cmd.ID, authsession.StatusRevoked); err != nil {
				return err
			}
		}
		if err := replaceUserRoles(lockedCtx, uc.roles, oldUser.Username, cmd.RoleIDs); err != nil {
			return err
		}
		return cacheErr
	})
}

// ResetUserPassword resets a user's password to the domain-defined default password.
func (uc *UserUseCase) ResetUserPassword(ctx context.Context, id uint64) error {
	savedUser, err := uc.user.FindByID(ctx, id)
	if err != nil {
		return err
	}
	defaultPassword, err := savedUser.DefaultPassword()
	if err != nil {
		return err
	}
	hashPass, err := xbcrypt.HashAndSalt(defaultPassword)
	if err != nil {
		return err
	}

	if err = uc.mysql.Transaction(ctx, func(txCtx context.Context) error {
		return uc.user.UpdatePassword(txCtx, id, hashPass)
	}); err != nil {
		return err
	}

	cacheErr := deleteAuthSnapshot(ctx, uc.snapshot, id)
	err = uc.auth.RevokeUser(ctx, id, authsession.StatusRevoked)
	if err == nil {
		err = cacheErr
	}
	return err
}

// Login authenticates a user or refreshes an existing session.
func (uc *UserUseCase) Login(ctx context.Context, cmd LoginCommand) (LoginResult, error) {
	var (
		isRefresh bool
		issued    *authsession.IssueResult
	)
	if cmd.RefreshToken != "" {
		isRefresh = true
		result, err := uc.auth.Refresh(ctx, authsession.RefreshRequest{
			RefreshToken: cmd.RefreshToken,
			IP:           cmd.IP,
		})
		if err != nil {
			return LoginResult{}, err
		}
		issued = result
		cmd.Username = result.Auth.Username
	}
	if !isRefresh && cmd.UA == "" {
		return LoginResult{}, ErrLoginUARequired
	}

	savedUser, err := uc.findLoginUser(ctx, cmd.Username)
	if err != nil {
		return LoginResult{}, err
	}
	if err = savedUser.CanLogin(); err != nil {
		return LoginResult{}, err
	}
	if !isRefresh && !comparePassword(savedUser.PasswordHash, cmd.Password) {
		return LoginResult{}, userdomain.ErrLoginDenied
	}

	if !isRefresh {
		issued, err = uc.auth.Issue(ctx, authsession.IssueRequest{
			UserID:   savedUser.ID,
			Username: savedUser.Username,
			UserType: userdomain.TypePlatform,
			UA:       cmd.UA,
			IP:       cmd.IP,
		})
		if err != nil {
			return LoginResult{}, err
		}
	}

	now := time.Now()
	if err = uc.user.MarkLoggedIn(ctx, savedUser.ID, now, cmd.IP); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		Auth:         issued.Auth,
		UserType:     userdomain.TypePlatform,
		Menus:        savedUser.Menus(),
		ExpiresAt:    issued.ExpiresAt,
		AccessToken:  issued.AccessToken,
		RefreshToken: issued.RefreshToken,
	}, nil
}

// OnlineUsers returns the count of users currently marked online.
func (uc *UserUseCase) OnlineUsers(ctx context.Context) (int64, error) {
	return uc.user.CountOnline(ctx)
}

// MarkUserOnline refreshes the heartbeat timestamp for an authenticated user.
func (uc *UserUseCase) MarkUserOnline(ctx context.Context, userID uint64) error {
	return uc.user.MarkOnline(ctx, userID, time.Now())
}

// MarkUsersOffline updates display-only online status.
func (uc *UserUseCase) MarkUsersOffline(ctx context.Context, userIDs []uint64) error {
	return uc.user.MarkOffline(ctx, userIDs)
}

// ForceUsersOffline marks users offline and revokes their sessions.
func (uc *UserUseCase) ForceUsersOffline(ctx context.Context, userIDs []uint64) error {
	if err := uc.MarkUsersOffline(ctx, userIDs); err != nil {
		return err
	}
	for _, userID := range userIDs {
		if err := uc.auth.RevokeUser(ctx, userID, authsession.StatusKicked); err != nil {
			return err
		}
	}
	return nil
}

// OfflineExpiredUsers applies heartbeat timeout policy to online users.
func (uc *UserUseCase) OfflineExpiredUsers(ctx context.Context, cmd OfflineExpiredCommand) error {
	if !cmd.Enabled {
		return nil
	}
	seconds := cmd.Seconds
	if seconds <= 0 {
		seconds = 60 * (10 + 1)
	}
	before := time.Now().Add(-time.Second * time.Duration(seconds))
	userIDs, err := uc.user.FindHeartbeatExpiredIDs(ctx, before)
	if err != nil {
		return err
	}
	if len(userIDs) == 0 {
		return nil
	}
	if cmd.RevokeSession {
		return uc.ForceUsersOffline(ctx, userIDs)
	}
	return uc.MarkUsersOffline(ctx, userIDs)
}

func deleteAuthSnapshot(ctx context.Context, cache AuthSnapshotCache, id uint64) error {
	if cache == nil || id == 0 {
		return nil
	}
	return cache.DeleteUser(ctx, id)
}

func replaceUserRoles(ctx context.Context, binding RoleBinding, username string, roleIDs []uint64) error {
	if binding == nil {
		return nil
	}
	return binding.ReplaceUserRoles(ctx, username, roleIDs)
}

func (uc *UserUseCase) findLoginUser(ctx context.Context, username string) (*userdomain.User, error) {
	savedUser, err := uc.user.FindByUsername(ctx, username)
	if err == nil {
		return savedUser, nil
	}
	if !errors.Is(err, userdomain.ErrNotFound) {
		return nil, err
	}
	savedUser, err = uc.user.FindByPhone(ctx, username)
	if err == nil {
		return savedUser, nil
	}
	if errors.Is(err, userdomain.ErrNotFound) {
		return nil, userdomain.ErrLoginDenied
	}
	return nil, err
}

func comparePassword(hashed string, password string) bool {
	if hashed == "" || password == "" {
		return false
	}
	return xbcrypt.Compare(hashed, password)
}

func (uc *UserUseCase) withCreateLocks(ctx context.Context, fn func(context.Context) error) error {
	if uc.locks == nil {
		return fn(ctx)
	}
	return uc.locks.WithCreateLocks(ctx, fn)
}

func (uc *UserUseCase) withUpdateLock(ctx context.Context, fn func(context.Context) error) error {
	if uc.locks == nil {
		return fn(ctx)
	}
	return uc.locks.WithUpdateLock(ctx, fn)
}

func (uc *UserUseCase) checkCreateUser(ctx context.Context, cmd CreateUserCommand) error {
	if userdomain.IsBuiltinUsername(cmd.Username) {
		return userdomain.ErrBuiltinUsername
	}
	savedUser, err := uc.user.FindByUsername(ctx, cmd.Username)
	if err == nil && savedUser != nil {
		return userdomain.ErrUsernameExists
	}
	if err != nil && !errors.Is(err, userdomain.ErrNotFound) {
		return err
	}
	if cmd.Phone == "" {
		return nil
	}
	savedUser, err = uc.user.FindByPhone(ctx, cmd.Phone)
	if err == nil && savedUser != nil {
		return userdomain.ErrPhoneExists
	}
	if err != nil && !errors.Is(err, userdomain.ErrNotFound) {
		return err
	}
	return nil
}

func (uc *UserUseCase) checkUpdateUser(ctx context.Context, cmd UpdateUserCommand) error {
	if cmd.Phone == "" {
		return nil
	}
	savedUser, err := uc.user.FindByPhone(ctx, cmd.Phone)
	if err == nil && savedUser != nil && savedUser.ID != cmd.ID {
		return userdomain.ErrPhoneExists
	}
	if err != nil && !errors.Is(err, userdomain.ErrNotFound) {
		return err
	}
	return nil
}
