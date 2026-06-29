package service

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/logincrypto"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"github.com/gotomicro/ego/core/econf"
	"gorm.io/gorm"
)

func init() {
	conf, _ := os.Open("../../../../configs/user/local-live.toml")
	defer conf.Close()

	econf.LoadFromReader(conf, toml.Unmarshal)
}

func TestDeptFilterByIDs(t *testing.T) {
	depts := []*store.DeptModel{
		{Model: xorm.Model{ID: 1}, ParentID: store.DeptModelParentTop},
		{Model: xorm.Model{ID: 2}, ParentID: 1},
		{Model: xorm.Model{ID: 3}, ParentID: 2},
		{Model: xorm.Model{ID: 4}, ParentID: 1},
		{Model: xorm.Model{ID: 5}, ParentID: store.DeptModelParentTop},
	}
	trees := deptAssembleTree(depts)
	ids := map[uint64]struct{}{
		3: {},
	}
	ftrees := deptFilterByIDs(trees, ids)

	got := collectDeptIDs(ftrees)
	want := []uint64{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered dept ids = %v, want %v", got, want)
	}
}

func TestDeptIDsFromCode(t *testing.T) {
	got, err := deptIDsFromCode("1-2-3")
	if err != nil {
		t.Fatalf("deptIDsFromCode() error = %v", err)
	}
	want := []uint64{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deptIDsFromCode() = %v, want %v", got, want)
	}
}

func TestVisibleDeptCandidatesFiltersByScope(t *testing.T) {
	depts := map[uint64]*store.DeptModel{
		1: {Model: xorm.Model{ID: 1}, Code: "1", DeptName: "总部"},
		2: {Model: xorm.Model{ID: 2}, Code: "1-2", DeptName: "研发部"},
		3: {Model: xorm.Model{ID: 3}, Code: "1-2-3", DeptName: "一组"},
		4: {Model: xorm.Model{ID: 4}, Code: "1-4", DeptName: "财务部"},
	}
	svc := &DeptService{Options: Options{Dept: fakeDeptStore{depts: depts}}}
	scope := DataScope{UserID: 10, DeptID: 2, Level: DataScopeDeptAndSub, DeptIDs: []uint64{2, 3}}

	got, err := svc.visibleDeptCandidates(context.Background(), scope, "研")
	if err != nil {
		t.Fatalf("visibleDeptCandidates() error = %v", err)
	}
	gotNames := make([]string, 0, len(got))
	for _, dept := range got {
		gotNames = append(gotNames, dept.DeptName)
	}
	want := []string{"研发部"}
	if !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("visibleDeptCandidates() = %v, want %v", gotNames, want)
	}
}

func TestDeptServiceGetDeptChainPreservesHierarchyOrder(t *testing.T) {
	depts := map[uint64]*store.DeptModel{
		1: {Model: xorm.Model{ID: 1}, Code: "1", DeptName: "总部"},
		2: {Model: xorm.Model{ID: 2}, Code: "1-2", DeptName: "研发部"},
		3: {Model: xorm.Model{ID: 3}, Code: "1-2-3", DeptName: "一组"},
	}
	svc := &DeptService{Options: Options{Dept: fakeDeptStore{depts: depts}}}

	got, err := svc.GetDeptChain(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetDeptChain() error = %v", err)
	}
	gotNames := make([]string, 0, len(got))
	for _, dept := range got {
		gotNames = append(gotNames, dept.DeptName)
	}
	want := []string{"总部", "研发部", "一组"}
	if !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("GetDeptChain() names = %v, want %v", gotNames, want)
	}
}

func TestDeptServiceTopVisibleDeptsReturnsVisibleRoots(t *testing.T) {
	depts := map[uint64]*store.DeptModel{
		1: {Model: xorm.Model{ID: 1}, ParentID: store.DeptModelParentTop, Code: "1", DeptName: "总部", Priority: 1},
		2: {Model: xorm.Model{ID: 2}, ParentID: 1, Code: "1-2", DeptName: "研发部", Priority: 1},
		3: {Model: xorm.Model{ID: 3}, ParentID: 2, Code: "1-2-3", DeptName: "一组", Priority: 1},
		4: {Model: xorm.Model{ID: 4}, ParentID: store.DeptModelParentTop, Code: "4", DeptName: "财务部", Priority: 2},
	}
	svc := &DeptService{Options: Options{Dept: fakeDeptStore{depts: depts}}}
	scope := DataScope{UserID: 10, DeptID: 2, Level: DataScopeDeptAndSub, DeptIDs: []uint64{2, 3}}

	got, err := svc.topVisibleDepts(context.Background(), scope)
	if err != nil {
		t.Fatalf("topVisibleDepts() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("topVisibleDepts() len = %d, want 1", len(got))
	}
	if got[0].ID != 2 {
		t.Fatalf("topVisibleDepts()[0].ID = %d, want 2", got[0].ID)
	}
	if !got[0].HasChildToRPC() {
		t.Fatal("expected visible root to keep child nodes")
	}
	if got[0].Childs[0].ID != 3 {
		t.Fatalf("topVisibleDepts()[0].Childs[0].ID = %d, want 3", got[0].Childs[0].ID)
	}
}

func TestDeptServiceTopVisibleDeptsRebuildsVisibleSubtree(t *testing.T) {
	depts := map[uint64]*store.DeptModel{
		1: {Model: xorm.Model{ID: 1}, ParentID: store.DeptModelParentTop, Code: "1", DeptName: "总部", Priority: 1},
		2: {Model: xorm.Model{ID: 2}, ParentID: 1, Code: "1-2", DeptName: "研发部", Priority: 1},
		3: {Model: xorm.Model{ID: 3}, ParentID: 2, Code: "1-2-3", DeptName: "一组", Priority: 1},
		4: {Model: xorm.Model{ID: 4}, ParentID: 2, Code: "1-2-4", DeptName: "二组", Priority: 2},
		5: {Model: xorm.Model{ID: 5}, ParentID: 1, Code: "1-5", DeptName: "财务部", Priority: 2},
	}
	svc := &DeptService{Options: Options{Dept: fakeDeptStore{depts: depts}}}
	scope := DataScope{UserID: 10, DeptID: 2, Level: DataScopeDeptAndSub, DeptIDs: []uint64{2, 3, 4}}

	got, err := svc.topVisibleDepts(context.Background(), scope)
	if err != nil {
		t.Fatalf("topVisibleDepts() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("topVisibleDepts() roots = %#v, want only dept 2", got)
	}
	if len(got[0].Childs) != 2 {
		t.Fatalf("topVisibleDepts() childs len = %d, want 2", len(got[0].Childs))
	}
	if got[0].Childs[0].ID != 3 || got[0].Childs[1].ID != 4 {
		t.Fatalf("topVisibleDepts() childs = %#v, want [3 4]", got[0].Childs)
	}
}

type fakeDeptStore struct {
	depts map[uint64]*store.DeptModel
}

func (s fakeDeptStore) Add(context.Context, *store.DeptModel) error  { return nil }
func (s fakeDeptStore) Delete(context.Context, uint64) error         { return nil }
func (s fakeDeptStore) DeleteByIds(context.Context, []uint64) error  { return nil }
func (s fakeDeptStore) Update(context.Context, uint64, string) error { return nil }
func (s fakeDeptStore) UpdatePriority(context.Context, []store.DeptModel) error {
	return nil
}

func (s fakeDeptStore) GetSelf(_ context.Context, id uint64) (*store.DeptModel, error) {
	dept, ok := s.depts[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return dept, nil
}

func (s fakeDeptStore) Get(ctx context.Context, id uint64) (*store.DeptModel, error) {
	dept, ok := s.depts[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	node := s.subtree(dept)
	return &node, nil
}

func (s fakeDeptStore) GetByName(_ context.Context, name string) ([]*store.DeptModel, error) {
	out := make([]*store.DeptModel, 0)
	for _, dept := range s.depts {
		if name == "" || strings.Contains(dept.DeptName, name) {
			out = append(out, dept)
		}
	}
	return out, nil
}

func (s fakeDeptStore) GetByID(context.Context, uint64) ([]*store.DeptModel, error) {
	return nil, nil
}

func (s fakeDeptStore) GetByIDs(_ context.Context, ids []uint64) ([]*store.DeptModel, error) {
	out := make([]*store.DeptModel, 0, len(ids))
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if dept, ok := s.depts[id]; ok {
			out = append(out, dept)
		}
	}
	return out, nil
}

func (s fakeDeptStore) GetByCode(_ context.Context, code string) ([]*store.DeptModel, error) {
	out := make([]*store.DeptModel, 0)
	for _, dept := range s.depts {
		if strings.HasPrefix(dept.Code, code) {
			out = append(out, dept)
		}
	}
	return out, nil
}

func (s fakeDeptStore) GetSubtreeIDs(ctx context.Context, id uint64) ([]uint64, error) {
	dept, err := s.GetSelf(ctx, id)
	if err != nil {
		return nil, err
	}
	depts, err := s.GetByCode(ctx, dept.Code)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(depts))
	for _, dept := range depts {
		if dept != nil {
			ids = append(ids, dept.ID)
		}
	}
	return ids, nil
}

func (s fakeDeptStore) GetAncestorIDs(ctx context.Context, id uint64) ([]uint64, error) {
	dept, err := s.GetSelf(ctx, id)
	if err != nil {
		return nil, err
	}
	return deptIDsFromCode(dept.Code)
}
func (s fakeDeptStore) GetAll(context.Context) ([]*store.DeptModel, error) { return nil, nil }
func (s fakeDeptStore) GetTopAll(context.Context) ([]*store.DeptModel, error) {
	depts := make([]*store.DeptModel, 0, len(s.depts))
	for _, dept := range s.depts {
		depts = append(depts, dept)
	}
	trees := deptAssembleTree(depts)
	out := make([]*store.DeptModel, 0, len(trees))
	for i := range trees {
		node := trees[i]
		out = append(out, &node)
	}
	return out, nil
}

func (s fakeDeptStore) GetChilds(context.Context, uint64) ([]*store.DeptModel, error) {
	return nil, nil
}

func (s fakeDeptStore) CountByOption(context.Context, ...func(*gorm.DB) *gorm.DB) (int64, error) {
	return 0, nil
}

func (s fakeDeptStore) subtree(dept *store.DeptModel) store.DeptModel {
	node := *dept
	node.Childs = nil
	for _, child := range s.depts {
		if child.ParentID == dept.ID {
			node.Childs = append(node.Childs, s.subtree(child))
		}
	}
	return node
}

type warmupLoginCrypto struct {
	calls int
	err   error
}

func (c *warmupLoginCrypto) ChallengeFor(context.Context, string, string, string) (logincrypto.Challenge, error) {
	return logincrypto.Challenge{}, nil
}

func (c *warmupLoginCrypto) DecryptPayload(context.Context, logincrypto.DecryptRequest) (logincrypto.LoginPayload, error) {
	return logincrypto.LoginPayload{}, nil
}

func (c *warmupLoginCrypto) Health(context.Context) error {
	c.calls++
	return c.err
}

func TestUserServiceWarmupLoginCrypto(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{name: "success"},
		{name: "health error", err: errors.New("store unavailable"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crypto := &warmupLoginCrypto{err: tt.err}
			svc := &UserService{Options: Options{LoginCrypto: crypto}}

			err := svc.WarmupLoginCrypto(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("WarmupLoginCrypto() error = %v, wantErr %v", err, tt.wantErr)
			}
			if crypto.calls != 1 {
				t.Fatalf("Health calls = %d, want 1", crypto.calls)
			}
			if tt.wantErr && !errors.Is(err, tt.err) {
				t.Fatalf("WarmupLoginCrypto() error = %v, want wrap %v", err, tt.err)
			}
		})
	}
}

func collectDeptIDs(trees []store.DeptModel) []uint64 {
	ids := make([]uint64, 0)
	for _, tree := range trees {
		ids = append(ids, tree.ID)
		ids = append(ids, collectDeptIDs(tree.Childs)...)
	}
	return ids
}
