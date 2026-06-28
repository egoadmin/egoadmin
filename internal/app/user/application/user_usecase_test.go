package application

import (
	"context"
	"errors"
	"testing"
	"time"

	userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xbcrypt"
	"gorm.io/gorm"
)

func TestUserUseCase_ResetUserPassword(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{
		users: map[uint64]*userdomain.User{
			42: {
				ID:       42,
				Username: "alice",
				Phone:    "13800138000",
			},
		},
	}
	tx := &transactionRunner{}
	auth := &revokeAuth{}
	cache := &snapshotCache{}

	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Mysql:          tx,
		Auth:           auth,
		AuthSnapshot:   cache,
	})

	if err := uc.ResetUserPassword(context.Background(), 42); err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
	if !tx.called {
		t.Fatal("ResetUserPassword() did not run in transaction")
	}
	if repo.updatedID != 42 {
		t.Fatalf("updatedID = %d, want 42", repo.updatedID)
	}
	if !xbcrypt.Compare(repo.passwordHash, "138000") {
		t.Fatal("stored password hash does not match expected default password")
	}
	if cache.deletedID != 42 {
		t.Fatalf("cache deletedID = %d, want 42", cache.deletedID)
	}
	if auth.revokedID != 42 {
		t.Fatalf("auth revokedID = %d, want 42", auth.revokedID)
	}
	if auth.reason != authsession.StatusRevoked {
		t.Fatalf("auth reason = %v, want StatusRevoked", auth.reason)
	}
}

func TestUserUseCase_CreateUser(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{
		users:  map[uint64]*userdomain.User{},
		nextID: 100,
	}
	tx := &transactionRunner{}
	roles := &roleBinding{}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Mysql:          tx,
		RoleBinding:    roles,
	})

	result, err := uc.CreateUser(context.Background(), CreateUserCommand{
		Username: "alice",
		Name:     "Alice",
		Phone:    "13800138000",
		Gender:   userdomain.GenderFemale,
		Status:   userdomain.StatusValid,
		DeptID:   12,
		Remark:   "remark",
		RoleIDs:  []uint64{2, 3},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if result.ID != 100 {
		t.Fatalf("CreateUser() ID = %d, want 100", result.ID)
	}
	if !tx.called {
		t.Fatal("CreateUser() did not run in transaction")
	}
	created := repo.users[result.ID]
	if created == nil {
		t.Fatal("CreateUser() did not persist user")
	}
	if !xbcrypt.Compare(created.PasswordHash, "138000") {
		t.Fatal("stored password hash does not match expected default password")
	}
	if created.OnlineStatus != userdomain.OnlineStatusOffline {
		t.Fatalf("created OnlineStatus = %v, want offline", created.OnlineStatus)
	}
	if created.HeartbeatTime.IsZero() {
		t.Fatal("created HeartbeatTime is zero")
	}
	if roles.username != "alice" {
		t.Fatalf("role binding username = %q, want alice", roles.username)
	}
	if len(roles.roleIDs) != 2 || roles.roleIDs[0] != 2 || roles.roleIDs[1] != 3 {
		t.Fatalf("role binding roleIDs = %v, want [2 3]", roles.roleIDs)
	}
}

func TestUserUseCase_CreateUserRejectsDuplicateUsername(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{
		users: map[uint64]*userdomain.User{
			1: {
				ID:       1,
				Username: "alice",
				Phone:    "13800138000",
			},
		},
	}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Mysql:          &transactionRunner{},
	})

	_, err := uc.CreateUser(context.Background(), CreateUserCommand{
		Username: "alice",
		Phone:    "13900139000",
	})
	if !errors.Is(err, userdomain.ErrUsernameExists) {
		t.Fatalf("CreateUser() error = %v, want ErrUsernameExists", err)
	}
}

func TestUserUseCase_UpdateUser(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{
		users: map[uint64]*userdomain.User{
			42: {
				ID:       42,
				Username: "alice",
				Phone:    "13800138000",
				Status:   userdomain.StatusValid,
			},
		},
	}
	tx := &transactionRunner{}
	cache := &snapshotCache{}
	roles := &roleBinding{}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Mysql:          tx,
		Auth:           &revokeAuth{},
		AuthSnapshot:   cache,
		RoleBinding:    roles,
	})

	err := uc.UpdateUser(context.Background(), UpdateUserCommand{
		ID:     42,
		Name:   "Alice",
		Phone:  "13900139000",
		Gender: userdomain.GenderFemale,
		Status: userdomain.StatusValid,
		DeptID: 12,
		RoleIDs: []uint64{
			2,
			3,
		},
	})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if !tx.called {
		t.Fatal("UpdateUser() did not run in transaction")
	}
	updated := repo.users[42]
	if updated.Name != "Alice" || updated.Phone != "13900139000" || updated.DeptID != 12 {
		t.Fatalf("updated user = %+v, want updated name/phone/dept", updated)
	}
	if cache.deletedID != 42 {
		t.Fatalf("cache deletedID = %d, want 42", cache.deletedID)
	}
	if roles.username != "alice" {
		t.Fatalf("role binding username = %q, want alice", roles.username)
	}
	if len(roles.roleIDs) != 2 || roles.roleIDs[0] != 2 || roles.roleIDs[1] != 3 {
		t.Fatalf("role binding roleIDs = %v, want [2 3]", roles.roleIDs)
	}
}

func TestUserUseCase_UpdateUserRejectsDuplicatePhone(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{
		users: map[uint64]*userdomain.User{
			42: {
				ID:       42,
				Username: "alice",
				Phone:    "13800138000",
			},
			43: {
				ID:       43,
				Username: "bob",
				Phone:    "13900139000",
			},
		},
	}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Mysql:          &transactionRunner{},
	})

	err := uc.UpdateUser(context.Background(), UpdateUserCommand{
		ID:    42,
		Phone: "13900139000",
	})
	if !errors.Is(err, userdomain.ErrPhoneExists) {
		t.Fatalf("UpdateUser() error = %v, want ErrPhoneExists", err)
	}
}

func TestUserUseCase_UpdateUserRevokesWhenDisabled(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{
		users: map[uint64]*userdomain.User{
			42: {
				ID:       42,
				Username: "alice",
				Phone:    "13800138000",
				Status:   userdomain.StatusValid,
			},
		},
	}
	auth := &revokeAuth{}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Mysql:          &transactionRunner{},
		Auth:           auth,
		AuthSnapshot:   &snapshotCache{},
		RoleBinding:    &roleBinding{},
	})

	if err := uc.UpdateUser(context.Background(), UpdateUserCommand{
		ID:     42,
		Phone:  "13800138000",
		Status: userdomain.StatusInvalid,
	}); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if auth.revokedID != 42 {
		t.Fatalf("revokedID = %d, want 42", auth.revokedID)
	}
	if auth.reason != authsession.StatusRevoked {
		t.Fatalf("reason = %v, want StatusRevoked", auth.reason)
	}
}

func TestUserUseCase_LoginWithPassword(t *testing.T) {
	t.Parallel()

	hash, err := xbcrypt.HashAndSalt("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	repo := &resetPasswordRepo{
		users: map[uint64]*userdomain.User{
			42: {
				ID:           42,
				Username:     "alice",
				PasswordHash: hash,
				Status:       userdomain.StatusValid,
				RoleMenus:    []string{"dashboard,user", "user,role"},
			},
		},
	}
	auth := &revokeAuth{
		issueResult: &authsession.IssueResult{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Unix(200, 0),
			Auth:         &authsession.AuthContext{UserID: 42, Username: "alice"},
		},
	}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Auth:           auth,
	})

	result, err := uc.Login(context.Background(), LoginCommand{
		Username: "alice",
		Password: "correct-password",
		UA:       "browser",
		IP:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if auth.issueReq.UserID != 42 || auth.issueReq.Username != "alice" || auth.issueReq.UA != "browser" || auth.issueReq.IP != "127.0.0.1" {
		t.Fatalf("issue request = %+v, want alice session request", auth.issueReq)
	}
	if result.UserType != userdomain.TypePlatform {
		t.Fatalf("UserType = %d, want %d", result.UserType, userdomain.TypePlatform)
	}
	if result.Menus != "dashboard,user,role" {
		t.Fatalf("Menus = %q, want dashboard,user,role", result.Menus)
	}
	if result.AccessToken != "access-token" || result.RefreshToken != "refresh-token" {
		t.Fatalf("tokens = %q/%q, want access-token/refresh-token", result.AccessToken, result.RefreshToken)
	}
	if repo.loggedInID != 42 || repo.loggedInIP != "127.0.0.1" || repo.loggedInAt.IsZero() {
		t.Fatalf("logged in state = id:%d ip:%q at:%v", repo.loggedInID, repo.loggedInIP, repo.loggedInAt)
	}
}

func TestUserUseCase_LoginWithPhone(t *testing.T) {
	t.Parallel()

	hash, err := xbcrypt.HashAndSalt("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	repo := &resetPasswordRepo{
		users: map[uint64]*userdomain.User{
			42: {
				ID:           42,
				Username:     "alice",
				Phone:        "13800138000",
				PasswordHash: hash,
				Status:       userdomain.StatusValid,
			},
		},
	}
	auth := &revokeAuth{
		issueResult: &authsession.IssueResult{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Unix(200, 0),
			Auth:         &authsession.AuthContext{UserID: 42, Username: "alice"},
		},
	}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Auth:           auth,
	})

	if _, err := uc.Login(context.Background(), LoginCommand{
		Username: "13800138000",
		Password: "correct-password",
		UA:       "browser",
	}); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if auth.issueReq.Username != "alice" {
		t.Fatalf("issue username = %q, want alice", auth.issueReq.Username)
	}
}

func TestUserUseCase_LoginRefreshUsesSessionUsername(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{
		users: map[uint64]*userdomain.User{
			42: {
				ID:       42,
				Username: "alice",
				Status:   userdomain.StatusValid,
			},
		},
	}
	auth := &revokeAuth{
		refreshResult: &authsession.IssueResult{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresAt:    time.Unix(300, 0),
			Auth:         &authsession.AuthContext{UserID: 42, Username: "alice"},
		},
	}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Auth:           auth,
	})

	result, err := uc.Login(context.Background(), LoginCommand{
		RefreshToken: "old-refresh",
		IP:           "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if auth.refreshReq.RefreshToken != "old-refresh" || auth.refreshReq.IP != "127.0.0.1" {
		t.Fatalf("refresh request = %+v, want old-refresh and ip", auth.refreshReq)
	}
	if auth.issueReq.UserID != 0 {
		t.Fatalf("Issue() was called during refresh: %+v", auth.issueReq)
	}
	if result.AccessToken != "new-access" || result.RefreshToken != "new-refresh" {
		t.Fatalf("tokens = %q/%q, want new-access/new-refresh", result.AccessToken, result.RefreshToken)
	}
}

func TestUserUseCase_LoginFailuresUseDomainError(t *testing.T) {
	t.Parallel()

	hash, err := xbcrypt.HashAndSalt("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		cmd  LoginCommand
		repo *resetPasswordRepo
	}{
		{
			name: "unknown username and phone",
			cmd: LoginCommand{
				Username: "missing",
				Password: "correct-password",
				UA:       "browser",
			},
			repo: &resetPasswordRepo{users: map[uint64]*userdomain.User{}},
		},
		{
			name: "invalid account status",
			cmd: LoginCommand{
				Username: "alice",
				Password: "correct-password",
				UA:       "browser",
			},
			repo: &resetPasswordRepo{
				users: map[uint64]*userdomain.User{
					42: {
						ID:           42,
						Username:     "alice",
						PasswordHash: hash,
						Status:       userdomain.StatusInvalid,
					},
				},
			},
		},
		{
			name: "wrong password",
			cmd: LoginCommand{
				Username: "alice",
				Password: "wrong-password",
				UA:       "browser",
			},
			repo: &resetPasswordRepo{
				users: map[uint64]*userdomain.User{
					42: {
						ID:           42,
						Username:     "alice",
						PasswordHash: hash,
						Status:       userdomain.StatusValid,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := NewUserUseCase(UserOptions{
				UserRepository: tt.repo,
				Auth: &revokeAuth{
					issueResult: &authsession.IssueResult{
						Auth: &authsession.AuthContext{},
					},
				},
			})
			_, err := uc.Login(context.Background(), tt.cmd)
			if !errors.Is(err, userdomain.ErrLoginDenied) {
				t.Fatalf("Login() error = %v, want ErrLoginDenied", err)
			}
			if tt.repo.loggedInID != 0 {
				t.Fatalf("loggedInID = %d, want 0 for failed login", tt.repo.loggedInID)
			}
		})
	}
}

func TestUserUseCase_MarkUserOnline(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{users: map[uint64]*userdomain.User{
		42: {ID: 42},
	}}
	uc := NewUserUseCase(UserOptions{UserRepository: repo})

	if err := uc.MarkUserOnline(context.Background(), 42); err != nil {
		t.Fatalf("MarkUserOnline() error = %v", err)
	}
	if repo.onlineID != 42 || repo.onlineAt.IsZero() {
		t.Fatalf("online state = id:%d at:%v", repo.onlineID, repo.onlineAt)
	}
}

func TestUserUseCase_ForceUsersOfflineRevokesSessions(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{}
	auth := &revokeAuth{}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Auth:           auth,
	})

	if err := uc.ForceUsersOffline(context.Background(), []uint64{5, 6}); err != nil {
		t.Fatalf("ForceUsersOffline() error = %v", err)
	}
	if !equalUint64s(repo.offlineIDs, []uint64{5, 6}) {
		t.Fatalf("offline ids = %v, want [5 6]", repo.offlineIDs)
	}
	if !equalUint64s(auth.revokedIDs, []uint64{5, 6}) {
		t.Fatalf("revoked ids = %v, want [5 6]", auth.revokedIDs)
	}
	if auth.reason != authsession.StatusKicked {
		t.Fatalf("reason = %v, want StatusKicked", auth.reason)
	}
}

func TestUserUseCase_OfflineExpiredUsersMarksOfflineWithoutRevokingSessionByDefault(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{heartbeatExpiredIDs: []uint64{1, 2}}
	auth := &revokeAuth{}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Auth:           auth,
	})

	if err := uc.OfflineExpiredUsers(context.Background(), OfflineExpiredCommand{
		Enabled: true,
		Seconds: 30,
	}); err != nil {
		t.Fatalf("OfflineExpiredUsers() error = %v", err)
	}
	if repo.heartbeatBefore.IsZero() {
		t.Fatal("OfflineExpiredUsers() did not query heartbeat expiry")
	}
	if !equalUint64s(repo.offlineIDs, []uint64{1, 2}) {
		t.Fatalf("offline ids = %v, want [1 2]", repo.offlineIDs)
	}
	if len(auth.revokedIDs) != 0 {
		t.Fatalf("revoked ids = %v, want none", auth.revokedIDs)
	}
}

func TestUserUseCase_OfflineExpiredUsersCanRevokeSessions(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{heartbeatExpiredIDs: []uint64{3, 4}}
	auth := &revokeAuth{}
	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Auth:           auth,
	})

	if err := uc.OfflineExpiredUsers(context.Background(), OfflineExpiredCommand{
		Enabled:       true,
		Seconds:       45,
		RevokeSession: true,
	}); err != nil {
		t.Fatalf("OfflineExpiredUsers() error = %v", err)
	}
	if !equalUint64s(repo.offlineIDs, []uint64{3, 4}) {
		t.Fatalf("offline ids = %v, want [3 4]", repo.offlineIDs)
	}
	if !equalUint64s(auth.revokedIDs, []uint64{3, 4}) {
		t.Fatalf("revoked ids = %v, want [3 4]", auth.revokedIDs)
	}
}

func TestUserUseCase_OfflineExpiredUsersDisabledSkipsQuery(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{heartbeatExpiredIDs: []uint64{1}}
	uc := NewUserUseCase(UserOptions{UserRepository: repo})

	if err := uc.OfflineExpiredUsers(context.Background(), OfflineExpiredCommand{}); err != nil {
		t.Fatalf("OfflineExpiredUsers() error = %v", err)
	}
	if !repo.heartbeatBefore.IsZero() {
		t.Fatalf("heartbeatBefore = %v, want zero", repo.heartbeatBefore)
	}
	if len(repo.offlineIDs) != 0 {
		t.Fatalf("offline ids = %v, want none", repo.offlineIDs)
	}
}

func TestUserUseCase_ResetUserPasswordRejectsBuiltinUser(t *testing.T) {
	t.Parallel()

	repo := &resetPasswordRepo{
		users: map[uint64]*userdomain.User{
			1: {
				ID:       1,
				Username: userdomain.BuiltinRootUsername,
				Phone:    "13800138000",
			},
		},
	}
	tx := &transactionRunner{}

	uc := NewUserUseCase(UserOptions{
		UserRepository: repo,
		Mysql:          tx,
		Auth:           &revokeAuth{},
		AuthSnapshot:   &snapshotCache{},
	})

	err := uc.ResetUserPassword(context.Background(), 1)
	if !errors.Is(err, userdomain.ErrBuiltinPasswordReset) {
		t.Fatalf("ResetUserPassword() error = %v, want ErrBuiltinPasswordReset", err)
	}
	if tx.called {
		t.Fatal("ResetUserPassword() ran transaction for rejected built-in user")
	}
}

type resetPasswordRepo struct {
	users               map[uint64]*userdomain.User
	nextID              uint64
	updatedID           uint64
	passwordHash        string
	loggedInID          uint64
	loggedInAt          time.Time
	loggedInIP          string
	onlineID            uint64
	onlineAt            time.Time
	offlineIDs          []uint64
	heartbeatExpiredIDs []uint64
	heartbeatBefore     time.Time
	onlineCount         int64
}

func (r *resetPasswordRepo) NextID(context.Context) (uint64, error) {
	if r.nextID == 0 {
		r.nextID = 1
	}
	id := r.nextID
	r.nextID++
	return id, nil
}

func (r *resetPasswordRepo) Create(ctx context.Context, u *userdomain.User) error {
	id, err := r.NextID(ctx)
	if err != nil {
		return err
	}
	u.ID = id
	if r.users == nil {
		r.users = map[uint64]*userdomain.User{}
	}
	cp := *u
	cp.RoleIDs = append([]uint64(nil), u.RoleIDs...)
	r.users[u.ID] = &cp
	return nil
}

func (r *resetPasswordRepo) Save(context.Context, *userdomain.User) error {
	return nil
}

func (r *resetPasswordRepo) Update(_ context.Context, id uint64, u *userdomain.User) error {
	saved, ok := r.users[id]
	if !ok {
		return userdomain.ErrNotFound
	}
	saved.Name = u.Name
	saved.Phone = u.Phone
	saved.Gender = u.Gender
	saved.Status = u.Status
	saved.DeptID = u.DeptID
	saved.RoleIDs = append([]uint64(nil), u.RoleIDs...)
	return nil
}

func (r *resetPasswordRepo) FindByID(_ context.Context, id uint64) (*userdomain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, userdomain.ErrNotFound
	}
	return cloneUser(u), nil
}

func (r *resetPasswordRepo) FindByUsername(_ context.Context, username string) (*userdomain.User, error) {
	for _, u := range r.users {
		if u.Username == username {
			return cloneUser(u), nil
		}
	}
	return nil, userdomain.ErrNotFound
}

func (r *resetPasswordRepo) FindByPhone(_ context.Context, phone string) (*userdomain.User, error) {
	for _, u := range r.users {
		if u.Phone == phone {
			return cloneUser(u), nil
		}
	}
	return nil, userdomain.ErrNotFound
}

func (r *resetPasswordRepo) List(context.Context, userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	return nil, 0, nil
}

func (r *resetPasswordRepo) UpdatePassword(_ context.Context, id uint64, passwordHash string) error {
	r.updatedID = id
	r.passwordHash = passwordHash
	return nil
}

func (r *resetPasswordRepo) MarkLoggedIn(_ context.Context, id uint64, at time.Time, ip string) error {
	r.loggedInID = id
	r.loggedInAt = at
	r.loggedInIP = ip
	if saved, ok := r.users[id]; ok {
		saved.OnlineStatus = userdomain.OnlineStatusOnline
		saved.HeartbeatTime = at
	}
	return nil
}

func (r *resetPasswordRepo) MarkOnline(_ context.Context, id uint64, at time.Time) error {
	r.onlineID = id
	r.onlineAt = at
	if saved, ok := r.users[id]; ok {
		saved.OnlineStatus = userdomain.OnlineStatusOnline
		saved.HeartbeatTime = at
	}
	return nil
}

func (r *resetPasswordRepo) MarkOffline(_ context.Context, ids []uint64) error {
	r.offlineIDs = append([]uint64(nil), ids...)
	return nil
}

func (r *resetPasswordRepo) FindHeartbeatExpiredIDs(_ context.Context, before time.Time) ([]uint64, error) {
	r.heartbeatBefore = before
	return append([]uint64(nil), r.heartbeatExpiredIDs...), nil
}

func (r *resetPasswordRepo) CountOnline(context.Context) (int64, error) {
	return r.onlineCount, nil
}

func (r *resetPasswordRepo) Delete(context.Context, []uint64) error {
	return nil
}

func cloneUser(u *userdomain.User) *userdomain.User {
	if u == nil {
		return nil
	}
	cp := *u
	cp.RoleIDs = append([]uint64(nil), u.RoleIDs...)
	return &cp
}

type transactionRunner struct {
	called bool
}

func (r *transactionRunner) Migrate(context.Context, []any, []platformmysql.MigrationJoinTable) error {
	return nil
}

func (r *transactionRunner) Transaction(ctx context.Context, callback func(context.Context) error) error {
	r.called = true
	return callback(ctx)
}

func (r *transactionRunner) WithTx(context.Context) *gorm.DB {
	return nil
}

type revokeAuth struct {
	issueReq      authsession.IssueRequest
	issueResult   *authsession.IssueResult
	refreshReq    authsession.RefreshRequest
	refreshResult *authsession.IssueResult
	revokedID     uint64
	revokedIDs    []uint64
	reason        authsession.Status
}

func (a *revokeAuth) Issue(_ context.Context, req authsession.IssueRequest) (*authsession.IssueResult, error) {
	a.issueReq = req
	if a.issueResult != nil {
		return a.issueResult, nil
	}
	return &authsession.IssueResult{
		Auth: &authsession.AuthContext{
			UserID:   req.UserID,
			Username: req.Username,
			UserType: req.UserType,
			UA:       req.UA,
		},
	}, nil
}

func (a *revokeAuth) Refresh(_ context.Context, req authsession.RefreshRequest) (*authsession.IssueResult, error) {
	a.refreshReq = req
	if a.refreshResult != nil {
		return a.refreshResult, nil
	}
	return &authsession.IssueResult{
		Auth: &authsession.AuthContext{},
	}, nil
}

func (a *revokeAuth) ValidateAccessToken(context.Context, string) (*authsession.AuthContext, error) {
	return nil, nil
}

func (a *revokeAuth) Logout(context.Context, *authsession.AuthContext) error {
	return nil
}

func (a *revokeAuth) RevokeSession(context.Context, string, authsession.Status) error {
	return nil
}

func (a *revokeAuth) RevokeUser(_ context.Context, userID uint64, reason authsession.Status) error {
	a.revokedID = userID
	a.revokedIDs = append(a.revokedIDs, userID)
	a.reason = reason
	return nil
}

func (a *revokeAuth) RevokeUserWorkspace(context.Context, uint64, uint64, authsession.Status) error {
	return nil
}

func (a *revokeAuth) ExpiresAt() time.Time {
	return time.Time{}
}

type snapshotCache struct {
	deletedID uint64
}

func (c *snapshotCache) DeleteUser(_ context.Context, userID uint64) error {
	c.deletedID = userID
	return nil
}

type roleBinding struct {
	username string
	roleIDs  []uint64
}

func (b *roleBinding) ReplaceUserRoles(_ context.Context, username string, roleIDs []uint64) error {
	b.username = username
	b.roleIDs = append([]uint64(nil), roleIDs...)
	return nil
}

func equalUint64s(a []uint64, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
