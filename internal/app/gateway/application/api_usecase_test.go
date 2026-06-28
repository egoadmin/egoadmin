package application

import (
	"context"
	"reflect"
	"testing"

	apidomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/api"
	permissiondomain "github.com/egoadmin/egoadmin/internal/app/gateway/domain/permission"
	"github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"gorm.io/gorm"
)

type syncAPIMysql struct{}

func (syncAPIMysql) Migrate(context.Context, []any, []mysql.MigrationJoinTable) error {
	return nil
}

func (syncAPIMysql) Transaction(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

func (syncAPIMysql) WithTx(context.Context) *gorm.DB {
	return nil
}

type syncAPIRepository struct {
	apis       []*apidomain.API
	added      []*apidomain.API
	updated    []*apidomain.API
	deletedIDs []uint64
}

func (f *syncAPIRepository) CreateBatch(_ context.Context, apis []*apidomain.API) error {
	f.added = append(f.added, apis...)
	return nil
}

func (f *syncAPIRepository) Update(_ context.Context, id uint64, api *apidomain.API) error {
	api.ID = id
	f.updated = append(f.updated, api)
	return nil
}

func (f *syncAPIRepository) DeleteByIDs(_ context.Context, ids []uint64) error {
	f.deletedIDs = append(f.deletedIDs, ids...)
	return nil
}

func (f *syncAPIRepository) FindAll(context.Context) ([]*apidomain.API, error) {
	return f.apis, nil
}

func (f *syncAPIRepository) FindAllBySign(context.Context, []string) ([]*apidomain.API, error) {
	return nil, nil
}

type syncPolicyCleaner struct {
	deleted []permissiondomain.Policy
}

func (f *syncPolicyCleaner) DeletePermissionPolicies(_ context.Context, policies []permissiondomain.Policy) error {
	f.deleted = append(f.deleted, policies...)
	return nil
}

func TestAPIUseCase_SyncFromCatalog(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	kept := &apidomain.API{
		ID:       1,
		Signcode: "11111111111111111111111111111111",
		Name:     "OLDNAME",
		Path:     "USER.V1.USERSERVICE",
		Method:   "GETUSER",
	}
	deleted := &apidomain.API{
		ID:       2,
		Signcode: "22222222222222222222222222222222",
		Name:     "OLDMETHOD",
		Path:     "USER.V1.OLDSERVICE",
		Method:   "OLDMETHOD",
	}

	cleaner := &syncPolicyCleaner{}
	repo := &syncAPIRepository{apis: []*apidomain.API{kept, deleted}}
	uc := NewAPIUseCase(APIOptions{
		APIRepository: repo,
		Mysql:         syncAPIMysql{},
		PolicyCleaner: cleaner,
	})

	catalog := []byte(`{
  "version": 1,
  "apis": [
    {
      "signcode": "11111111111111111111111111111111",
      "method": "GetUser",
      "path": "USER.V1.USERSERVICE",
      "action": "GETUSER"
    },
    {
      "signcode": "33333333333333333333333333333333",
      "method": "AddUser",
      "path": "USER.V1.USERSERVICE",
      "action": "ADDUSER"
    }
  ]
}`)

	if err := uc.SyncFromCatalog(ctx, catalog); err != nil {
		t.Fatalf("SyncFromCatalog() error = %v", err)
	}

	if len(repo.added) != 1 || repo.added[0].Signcode != "33333333333333333333333333333333" {
		t.Fatalf("added APIs = %+v", repo.added)
	}
	if len(repo.updated) != 1 || repo.updated[0].ID != kept.ID || repo.updated[0].Name != "GETUSER" {
		t.Fatalf("updated APIs = %+v", repo.updated)
	}
	if repo.updated[0].Method != "GetUser" {
		t.Fatalf("updated API method = %q, want GetUser", repo.updated[0].Method)
	}
	if !reflect.DeepEqual(repo.deletedIDs, []uint64{deleted.ID}) {
		t.Fatalf("deleted API ids = %v", repo.deletedIDs)
	}
	if !reflect.DeepEqual(cleaner.deleted, []permissiondomain.Policy{{Service: deleted.Path, Method: deleted.Method}}) {
		t.Fatalf("deleted permission policies = %+v", cleaner.deleted)
	}
}
