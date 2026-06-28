package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/app/user/application"
	userdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/user"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/platform/config"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

type offlineUserRepo struct {
	expiredBefore time.Time
	expiredIDs    []uint64
	offlineIDs    []uint64
	users         map[uint64]*userdomain.User
}

func (r *offlineUserRepo) NextID(context.Context) (uint64, error) { return 0, nil }
func (r *offlineUserRepo) Create(context.Context, *userdomain.User) error {
	return nil
}
func (r *offlineUserRepo) Save(context.Context, *userdomain.User) error { return nil }
func (r *offlineUserRepo) Update(context.Context, uint64, *userdomain.User) error {
	return nil
}
func (r *offlineUserRepo) FindByID(context.Context, uint64) (*userdomain.User, error) {
	return nil, userdomain.ErrNotFound
}
func (r *offlineUserRepo) FindByUsername(context.Context, string) (*userdomain.User, error) {
	return nil, userdomain.ErrNotFound
}
func (r *offlineUserRepo) FindByPhone(context.Context, string) (*userdomain.User, error) {
	return nil, userdomain.ErrNotFound
}
func (r *offlineUserRepo) List(context.Context, userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	return nil, 0, nil
}
func (r *offlineUserRepo) UpdatePassword(context.Context, uint64, string) error {
	return nil
}
func (r *offlineUserRepo) MarkLoggedIn(context.Context, uint64, time.Time, string) error {
	return nil
}
func (r *offlineUserRepo) MarkOnline(context.Context, uint64, time.Time) error {
	return nil
}
func (r *offlineUserRepo) MarkOffline(_ context.Context, ids []uint64) error {
	r.offlineIDs = append([]uint64(nil), ids...)
	return nil
}
func (r *offlineUserRepo) FindHeartbeatExpiredIDs(_ context.Context, before time.Time) ([]uint64, error) {
	r.expiredBefore = before
	return append([]uint64(nil), r.expiredIDs...), nil
}
func (r *offlineUserRepo) CountOnline(context.Context) (int64, error) { return 0, nil }
func (r *offlineUserRepo) Delete(context.Context, []uint64) error     { return nil }

type offlineUserStore struct {
	users []*store.UserModel
}

func (s offlineUserStore) Add(context.Context, *store.UserModel) error                { return nil }
func (s offlineUserStore) BatchAdd(context.Context, []*store.UserModel) error         { return nil }
func (s offlineUserStore) Delete(context.Context, []uint64) error                     { return nil }
func (s offlineUserStore) Update(context.Context, uint64, *store.UserModel) error     { return nil }
func (s offlineUserStore) UpdateBase(context.Context, uint64, *store.UserModel) error { return nil }
func (s offlineUserStore) UpdateBaseWithoutHook(context.Context, uint64, *store.UserModel) error {
	return nil
}
func (s offlineUserStore) UpdateBaseWithoutHookAndTx(context.Context, uint64, *store.UserModel) error {
	return nil
}
func (s offlineUserStore) UpdatePass(context.Context, uint64, string) error { return nil }
func (s offlineUserStore) Get(context.Context, uint64) (*store.UserModel, error) {
	return &store.UserModel{
		Username:   store.UserModelUsernameAdmin,
		UserStatus: store.UserModelStatusValid,
		UserType:   store.UserModelTypePlatform,
		DeptID:     1,
		Roles:      []store.RoleModel{{DataPerm: store.RoleModelDataPermAll}},
	}, nil
}
func (s offlineUserStore) GetAuthSnapshot(context.Context, uint64) (*store.UserAuthSnapshot, error) {
	return nil, nil
}
func (s offlineUserStore) GetByUsername(context.Context, string) (*store.UserModel, error) {
	return nil, nil
}
func (s offlineUserStore) GetByPhone(context.Context, string) (*store.UserModel, error) {
	return nil, nil
}
func (s offlineUserStore) GetList(context.Context, store.UserModelGetListOption, ...func(*gorm.DB) *gorm.DB) ([]*store.UserModel, int64, error) {
	return nil, 0, nil
}
func (s offlineUserStore) GetByIds(context.Context, []uint64) ([]*store.UserModel, error) {
	return s.users, nil
}
func (s offlineUserStore) GetByDeptIds(context.Context, []uint64) ([]*store.UserModel, error) {
	return nil, nil
}
func (s offlineUserStore) CountByDeptIds(context.Context, []uint64) (int64, error) { return 0, nil }
func (s offlineUserStore) GetByUsernames(context.Context, []string) ([]*store.UserModel, error) {
	return nil, nil
}
func (s offlineUserStore) GetByNames(context.Context, []string) ([]*store.UserModel, error) {
	return nil, nil
}
func (s offlineUserStore) GetByPhones(context.Context, []string) ([]*store.UserModel, error) {
	return nil, nil
}
func (s offlineUserStore) GetHeartbeatExpiredUids(context.Context, int64) ([]uint64, error) {
	return nil, nil
}
func (s offlineUserStore) BatchOffline(context.Context, []uint64) error { return nil }
func (s offlineUserStore) CountOnline(context.Context) (int64, error)   { return 0, nil }
func (s offlineUserStore) CountByOption(context.Context, func(*gorm.DB) *gorm.DB) (int64, error) {
	return 0, nil
}
func (s offlineUserStore) CountByRole(context.Context, uint64) (int64, error) { return 0, nil }
func (s offlineUserStore) GetByRoleID(context.Context, uint64) ([]*store.UserModel, error) {
	return nil, nil
}

type offlineAuthSession struct {
	revoked []uint64
	err     error
}

func (s *offlineAuthSession) Issue(context.Context, authsession.IssueRequest) (*authsession.IssueResult, error) {
	return nil, errors.New("not implemented")
}
func (s *offlineAuthSession) Refresh(context.Context, authsession.RefreshRequest) (*authsession.IssueResult, error) {
	return nil, errors.New("not implemented")
}
func (s *offlineAuthSession) ValidateAccessToken(context.Context, string) (*authsession.AuthContext, error) {
	return nil, errors.New("not implemented")
}
func (s *offlineAuthSession) Logout(context.Context, *authsession.AuthContext) error {
	return errors.New("not implemented")
}
func (s *offlineAuthSession) RevokeSession(context.Context, string, authsession.Status) error {
	return errors.New("not implemented")
}
func (s *offlineAuthSession) RevokeUser(_ context.Context, userID uint64, reason authsession.Status) error {
	if reason != authsession.StatusKicked {
		return errors.New("unexpected revoke reason")
	}
	s.revoked = append(s.revoked, userID)
	return s.err
}
func (s *offlineAuthSession) RevokeUserWorkspace(context.Context, uint64, uint64, authsession.Status) error {
	return errors.New("not implemented")
}
func (s *offlineAuthSession) ExpiresAt() time.Time { return time.Now().Add(time.Hour) }

func TestUserService_OfflineUserMarksOfflineWithoutRevokingSessionByDefault(t *testing.T) {
	ctx := context.Background()
	conf := config.New(config.WithService(config.ServiceUser), config.WithEnvPrefix(""))
	userConf := conf.User()
	userConf.HeartbeatOfflineEnabled = true
	userConf.HeartbeatOfflineSeconds = 30
	userConf.RevokeSessionOnHeartbeatOffline = false
	conf.SetUserForTest(userConf)
	users := &offlineUserRepo{expiredIDs: []uint64{1, 2}}
	auth := &offlineAuthSession{}
	uc := application.NewUserUseCase(application.UserOptions{UserRepository: users, Auth: auth})
	svc := NewUserService(Options{Conf: conf, Auth: auth, UserUseCase: uc})

	if err := svc.OfflineUser(ctx); err != nil {
		t.Fatal(err)
	}
	if users.expiredBefore.IsZero() {
		t.Fatal("OfflineUser() did not query expired heartbeat users")
	}
	if !reflect.DeepEqual(users.offlineIDs, []uint64{1, 2}) {
		t.Fatalf("offline uids = %v, want [1 2]", users.offlineIDs)
	}
	if len(auth.revoked) != 0 {
		t.Fatalf("revoked users = %v, want none", auth.revoked)
	}
}

func TestUserService_OfflineUserCanRevokeSessionsWhenConfigured(t *testing.T) {
	ctx := context.Background()
	conf := config.New(config.WithService(config.ServiceUser), config.WithEnvPrefix(""))
	userConf := conf.User()
	userConf.HeartbeatOfflineEnabled = true
	userConf.HeartbeatOfflineSeconds = 45
	userConf.RevokeSessionOnHeartbeatOffline = true
	conf.SetUserForTest(userConf)
	users := &offlineUserRepo{expiredIDs: []uint64{3, 4}}
	auth := &offlineAuthSession{}
	uc := application.NewUserUseCase(application.UserOptions{UserRepository: users, Auth: auth})
	svc := NewUserService(Options{Conf: conf, Auth: auth, UserUseCase: uc})

	if err := svc.OfflineUser(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(users.offlineIDs, []uint64{3, 4}) {
		t.Fatalf("offline uids = %v, want [3 4]", users.offlineIDs)
	}
	if !reflect.DeepEqual(auth.revoked, []uint64{3, 4}) {
		t.Fatalf("revoked users = %v, want [3 4]", auth.revoked)
	}
}

func TestUserService_ForceOfflineUsersRevokesSessions(t *testing.T) {
	ctx := authsession.NewContext(context.Background(), &authsession.AuthContext{
		UserID:         1,
		Username:       store.UserModelUsernameAdmin,
		DeptID:         1,
		IsBuiltinAdmin: true,
	})
	users := &offlineUserRepo{}
	auth := &offlineAuthSession{}
	uc := application.NewUserUseCase(application.UserOptions{UserRepository: users, Auth: auth})
	svc := NewUserService(Options{
		Auth:        auth,
		UserUseCase: uc,
		User:        offlineUserStore{users: []*store.UserModel{{Model: xorm.Model{ID: 5}, DeptID: 2}}},
	})

	if err := svc.ForceOfflineUsers(ctx, []uint64{5}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(users.offlineIDs, []uint64{5}) {
		t.Fatalf("offline uids = %v, want [5]", users.offlineIDs)
	}
	if !reflect.DeepEqual(auth.revoked, []uint64{5}) {
		t.Fatalf("revoked users = %v, want [5]", auth.revoked)
	}
}
