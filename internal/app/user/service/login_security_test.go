package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/elib/pkg/util/xbcrypt"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gorm.io/gorm"
)

type loginUserStore struct {
	byUsername    *store.UserModel
	byUsernameErr error
	byPhone       *store.UserModel
	byPhoneErr    error
	authSnapshot  *store.UserAuthSnapshot
	authErr       error
	authCalls     int
}

func (s loginUserStore) Add(context.Context, *store.UserModel) error { return nil }
func (s loginUserStore) BatchAdd(context.Context, []*store.UserModel) error {
	return nil
}
func (s loginUserStore) Delete(context.Context, []uint64) error { return nil }
func (s loginUserStore) Update(context.Context, uint64, *store.UserModel) error {
	return nil
}

func (s loginUserStore) UpdateBase(context.Context, uint64, *store.UserModel) error {
	return nil
}

func (s loginUserStore) UpdateBaseWithoutHook(context.Context, uint64, *store.UserModel) error {
	return nil
}

func (s loginUserStore) UpdateBaseWithoutHookAndTx(context.Context, uint64, *store.UserModel) error {
	return nil
}
func (s loginUserStore) UpdatePass(context.Context, uint64, string) error { return nil }
func (s loginUserStore) Get(context.Context, uint64) (*store.UserModel, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *loginUserStore) GetAuthSnapshot(context.Context, uint64) (*store.UserAuthSnapshot, error) {
	s.authCalls++
	return s.authSnapshot, s.authErr
}

func (s loginUserStore) GetByUsername(context.Context, string) (*store.UserModel, error) {
	return s.byUsername, s.byUsernameErr
}

func (s loginUserStore) GetByPhone(context.Context, string) (*store.UserModel, error) {
	return s.byPhone, s.byPhoneErr
}

func (s loginUserStore) GetList(context.Context, store.UserModelGetListOption, ...func(*gorm.DB) *gorm.DB) ([]*store.UserModel, int64, error) {
	return nil, 0, nil
}

func (s loginUserStore) GetByIds(context.Context, []uint64) ([]*store.UserModel, error) {
	return nil, nil
}

func (s loginUserStore) GetByDeptIds(context.Context, []uint64) ([]*store.UserModel, error) {
	return nil, nil
}
func (s loginUserStore) CountByDeptIds(context.Context, []uint64) (int64, error) { return 0, nil }
func (s loginUserStore) GetByUsernames(context.Context, []string) ([]*store.UserModel, error) {
	return nil, nil
}

func (s loginUserStore) GetByNames(context.Context, []string) ([]*store.UserModel, error) {
	return nil, nil
}

func (s loginUserStore) GetByPhones(context.Context, []string) ([]*store.UserModel, error) {
	return nil, nil
}

func (s loginUserStore) GetHeartbeatExpiredUids(context.Context, int64) ([]uint64, error) {
	return nil, nil
}
func (s loginUserStore) BatchOffline(context.Context, []uint64) error { return nil }
func (s loginUserStore) CountOnline(context.Context) (int64, error)   { return 0, nil }
func (s loginUserStore) CountByOption(context.Context, func(*gorm.DB) *gorm.DB) (int64, error) {
	return 0, nil
}

func (s loginUserStore) CountByRole(context.Context, uint64) (int64, error) {
	return 0, nil
}
func (s loginUserStore) GetByRoleID(context.Context, uint64) ([]*store.UserModel, error) {
	return nil, nil
}

type memoryAuthSnapshotCache struct {
	mu      sync.Mutex
	items   map[string]authUserSnapshot
	deletes []string
	getErr  error
	setErr  error
	delErr  error
}

func (c *memoryAuthSnapshotCache) Get(ctx context.Context, key string, val *authUserSnapshot) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.getErr != nil {
		return c.getErr
	}
	item, ok := c.items[key]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	*val = item
	return nil
}

func (c *memoryAuthSnapshotCache) Set(ctx context.Context, key string, val authUserSnapshot, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.setErr != nil {
		return c.setErr
	}
	if c.items == nil {
		c.items = make(map[string]authUserSnapshot)
	}
	c.items[key] = val
	return nil
}

func (c *memoryAuthSnapshotCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deletes = append(c.deletes, key)
	if c.delErr != nil {
		return c.delErr
	}
	delete(c.items, key)
	return nil
}

func TestLoginPasswordFailuresUseGenericMessage(t *testing.T) {
	hash, err := xbcrypt.HashAndSalt("correct-password")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		user store.UserInterface
	}{
		{
			name: "unknown username and phone",
			user: &loginUserStore{
				byUsernameErr: gorm.ErrRecordNotFound,
				byPhoneErr:    gorm.ErrRecordNotFound,
			},
		},
		{
			name: "invalid account status",
			user: &loginUserStore{
				byUsername: &store.UserModel{
					Username:   "admin",
					Password:   hash,
					UserStatus: store.UserModelStatusInvalid,
				},
			},
		},
		{
			name: "wrong password",
			user: &loginUserStore{
				byUsername: &store.UserModel{
					Username:   "admin",
					Password:   hash,
					UserStatus: store.UserModelStatusValid,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &UserService{Options: Options{User: tt.user}}
			_, _, err := svc.Login(context.Background(), "admin", "wrong-password", "browser", "", "127.0.0.1", nil)
			if err == nil {
				t.Fatal("expected login to fail")
			}
			if !strings.Contains(err.Error(), loginFailedMessage) {
				t.Fatalf("Login() error = %q, want generic login failure", err.Error())
			}
		})
	}
}

func TestAuthContextValidatorCachesUserSnapshot(t *testing.T) {
	ctx := context.Background()
	cache := &memoryAuthSnapshotCache{}
	user := &loginUserStore{
		authSnapshot: &store.UserAuthSnapshot{
			ID:         1,
			Username:   store.UserModelUsernameAdmin,
			UserStatus: store.UserModelStatusValid,
			UserType:   store.UserModelTypePlatform,
		},
	}
	validator := newAuthContextValidator(cache, user)

	for i := 0; i < 2; i++ {
		auth := &authsession.AuthContext{UserID: 1}
		if err := validator.ValidateAuthContext(ctx, auth); err != nil {
			t.Fatal(err)
		}
		if auth.Username != store.UserModelUsernameAdmin || !auth.IsBuiltinAdmin || auth.UserType != store.UserModelTypePlatform {
			t.Fatalf("auth context not filled from snapshot: %+v", auth)
		}
	}

	if user.authCalls != 1 {
		t.Fatalf("GetAuthSnapshot calls = %d, want 1", user.authCalls)
	}
}

func TestAuthContextValidatorRejectsInvalidCachedUser(t *testing.T) {
	ctx := context.Background()
	cache := &memoryAuthSnapshotCache{
		items: map[string]authUserSnapshot{
			authUserSnapshotCacheKey(1): {
				ID:         1,
				Username:   "disabled",
				UserStatus: store.UserModelStatusInvalid,
				UserType:   store.UserModelTypePlatform,
			},
		},
	}
	validator := newAuthContextValidator(cache, &loginUserStore{})

	err := validator.ValidateAuthContext(ctx, &authsession.AuthContext{UserID: 1})
	if err == nil || !strings.Contains(err.Error(), "用户已失效") {
		t.Fatalf("ValidateAuthContext() error = %v, want invalid user error", err)
	}
}

func TestDeleteAuthUserSnapshotCache(t *testing.T) {
	ctx := context.Background()
	cache := &memoryAuthSnapshotCache{
		items: map[string]authUserSnapshot{
			authUserSnapshotCacheKey(1): {ID: 1},
		},
	}

	if err := deleteAuthUserSnapshotCache(ctx, cache, 0, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.items[authUserSnapshotCacheKey(1)]; ok {
		t.Fatal("snapshot cache was not deleted")
	}
	if len(cache.deletes) != 1 || cache.deletes[0] != authUserSnapshotCacheKey(1) {
		t.Fatalf("deleted keys = %v", cache.deletes)
	}
}

func TestPasswordRequestMessagesUseCipherOnly(t *testing.T) {
	tests := []struct {
		name       string
		message    protoreflect.ProtoMessage
		forbidden  []protoreflect.Name
		requiredBy []protoreflect.Name
	}{
		{
			name:       "login",
			message:    &userv1.LoginRequest{},
			forbidden:  []protoreflect.Name{"password", "old_password"},
			requiredBy: []protoreflect.Name{"password_cipher", "key_id", "challenge_id"},
		},
		{
			name:       "center edit info",
			message:    &userv1.EditCenterInfoRequest{},
			forbidden:  []protoreflect.Name{"password", "old_password"},
			requiredBy: []protoreflect.Name{"password_cipher", "key_id", "challenge_id"},
		},
		{
			name:       "center edit password",
			message:    &userv1.EditCenterPasswordRequest{},
			forbidden:  []protoreflect.Name{"password", "old_password"},
			requiredBy: []protoreflect.Name{"password_cipher", "key_id", "challenge_id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := tt.message.ProtoReflect().Descriptor().Fields()
			for _, field := range tt.forbidden {
				if fields.ByName(field) != nil {
					t.Fatalf("%s must not expose legacy password field %q", tt.name, field)
				}
			}
			for _, field := range tt.requiredBy {
				if fields.ByName(field) == nil {
					t.Fatalf("%s missing RSA challenge field %q", tt.name, field)
				}
			}
		})
	}
}

func TestPasswordSecuritySourceDoesNotReintroduceLegacySchemes(t *testing.T) {
	root := findRepoRoot(t)
	tests := []struct {
		name      string
		dir       string
		forbidden []string
	}{
		{
			name: "user service auth code",
			dir:  "internal/app/user",
			forbidden: []string{
				"crypto/md5",
				"md5.Sum",
				"md5(",
				"OPAQUE",
				"StartOpaque",
				"FinishOpaque",
				"passwordauth",
			},
		},
		{
			name: "frontend auth code",
			dir:  "web/src",
			forbidden: []string{
				"spark-md5",
				"js-md5",
				"crypto-js/md5",
				"MD5(",
				"md5(",
				"OPAQUE",
				"StartOpaque",
				"FinishOpaque",
				"passwordauth",
			},
		},
		{
			name: "proto contracts",
			dir:  "api/proto/user/v1",
			forbidden: []string{
				"OPAQUE",
				"StartOpaque",
				"FinishOpaque",
				"passwordauth",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := filepath.WalkDir(filepath.Join(root, tt.dir), func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					base := d.Name()
					if base == "dist" || base == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(path, "_test.go") {
					return nil
				}
				ext := filepath.Ext(path)
				if ext != ".go" && ext != ".proto" && ext != ".ts" && ext != ".vue" {
					return nil
				}
				raw, er := os.ReadFile(path)
				if er != nil {
					return er
				}
				content := string(raw)
				for _, token := range tt.forbidden {
					if strings.Contains(content, token) {
						t.Fatalf("%s contains forbidden legacy auth token %q", path, token)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err = os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found")
		}
		wd = parent
	}
}
