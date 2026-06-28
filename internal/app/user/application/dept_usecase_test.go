package application

import (
	"context"
	"errors"
	"testing"

	deptdomain "github.com/egoadmin/egoadmin/internal/app/user/domain/dept"
)

func TestDeptUseCase_CreateDept(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{nextID: 100}
	tx := &transactionRunner{}
	locks := &deptLocks{}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          tx,
		DeptLocks:      locks,
	})

	result, err := uc.CreateDept(context.Background(), CreateDeptCommand{
		Name:   "总部",
		Leader: "alice",
	})
	if err != nil {
		t.Fatalf("CreateDept() error = %v", err)
	}
	if result.ID != 100 {
		t.Fatalf("CreateDept() ID = %d, want 100", result.ID)
	}
	if !tx.called {
		t.Fatal("CreateDept() did not run in transaction")
	}
	if locks.createCalls != 1 {
		t.Fatalf("create locks = %d, want 1", locks.createCalls)
	}
	created := repo.depts[result.ID]
	if created.Code != "100" || created.Level != 1 || created.Priority != 1 || created.Status != deptdomain.StatusValid {
		t.Fatalf("created dept = %+v, want root code/level/priority/status", created)
	}
}

func TestDeptUseCase_CreateChildDept(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		nextID: 200,
		depts: map[uint64]*deptdomain.Dept{
			100: {
				ID:    100,
				Code:  "100",
				Name:  "总部",
				Level: 1,
			},
			101: {
				ID:       101,
				Code:     "100-101",
				ParentID: 100,
				Name:     "研发一部",
				Level:    2,
				Priority: 1,
			},
		},
	}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          &transactionRunner{},
	})

	result, err := uc.CreateDept(context.Background(), CreateDeptCommand{
		ParentID: 100,
		Name:     "研发二部",
	})
	if err != nil {
		t.Fatalf("CreateDept() error = %v", err)
	}
	created := repo.depts[result.ID]
	if created.Code != "100-200" || created.Level != 2 || created.Priority != 2 {
		t.Fatalf("created child = %+v, want code 100-200 level 2 priority 2", created)
	}
}

func TestDeptUseCase_CreateDeptRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		depts: map[uint64]*deptdomain.Dept{
			1: {ID: 1, ParentID: 0, Name: "总部"},
		},
	}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          &transactionRunner{},
	})

	_, err := uc.CreateDept(context.Background(), CreateDeptCommand{Name: "总部"})
	if !errors.Is(err, deptdomain.ErrNameExists) {
		t.Fatalf("CreateDept() error = %v, want ErrNameExists", err)
	}
	var nameExists deptdomain.NameExistsError
	if !errors.As(err, &nameExists) || nameExists.Name != "总部" {
		t.Fatalf("CreateDept() duplicate error = %#v, want name 总部", err)
	}
}

func TestDeptUseCase_CreateDeptRejectsMaxLevel(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		nextID: 200,
		depts: map[uint64]*deptdomain.Dept{
			100: {
				ID:    100,
				Code:  "1-2-3-4-100",
				Name:  "五级组织",
				Level: deptdomain.MaxLevel,
			},
		},
	}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          &transactionRunner{},
	})

	_, err := uc.CreateDept(context.Background(), CreateDeptCommand{
		ParentID: 100,
		Name:     "超层级组织",
	})
	if !errors.Is(err, deptdomain.ErrMaxLevel) {
		t.Fatalf("CreateDept() error = %v, want ErrMaxLevel", err)
	}
}

func TestDeptUseCase_UpdateDept(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		depts: map[uint64]*deptdomain.Dept{
			10: {ID: 10, ParentID: 1, Name: "研发部"},
		},
	}
	tx := &transactionRunner{}
	locks := &deptLocks{}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          tx,
		DeptLocks:      locks,
	})

	if err := uc.UpdateDept(context.Background(), 10, "技术部"); err != nil {
		t.Fatalf("UpdateDept() error = %v", err)
	}
	if !tx.called {
		t.Fatal("UpdateDept() did not run in transaction")
	}
	if locks.updateCalls != 1 {
		t.Fatalf("update locks = %d, want 1", locks.updateCalls)
	}
	if repo.depts[10].Name != "技术部" {
		t.Fatalf("dept name = %q, want 技术部", repo.depts[10].Name)
	}
}

func TestDeptUseCase_UpdateDeptRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		depts: map[uint64]*deptdomain.Dept{
			10: {ID: 10, ParentID: 1, Name: "研发部"},
			11: {ID: 11, ParentID: 1, Name: "财务部"},
		},
	}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          &transactionRunner{},
	})

	err := uc.UpdateDept(context.Background(), 10, "财务部")
	if !errors.Is(err, deptdomain.ErrNameExists) {
		t.Fatalf("UpdateDept() error = %v, want ErrNameExists", err)
	}
}

func TestDeptUseCase_UpdatePriorityDept(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		depts: map[uint64]*deptdomain.Dept{
			10: {ID: 10, ParentID: 1, Name: "研发部", Priority: 1},
			11: {ID: 11, ParentID: 1, Name: "财务部", Priority: 2},
		},
	}
	tx := &transactionRunner{}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          tx,
		DeptLocks:      &deptLocks{},
	})

	err := uc.UpdatePriorityDept(context.Background(), UpdatePriorityCommand{
		Items: []deptdomain.PriorityUpdate{
			{ID: 10, Priority: 2},
			{ID: 11, Priority: 1},
		},
	})
	if err != nil {
		t.Fatalf("UpdatePriorityDept() error = %v", err)
	}
	if !tx.called {
		t.Fatal("UpdatePriorityDept() did not run in transaction")
	}
	if repo.depts[10].Priority != 2 || repo.depts[11].Priority != 1 {
		t.Fatalf("priorities = %d/%d, want 2/1", repo.depts[10].Priority, repo.depts[11].Priority)
	}
}

func TestDeptUseCase_UpdatePriorityDeptRejectsChangedData(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		depts: map[uint64]*deptdomain.Dept{
			10: {ID: 10, ParentID: 1, Name: "研发部", Priority: 1},
			11: {ID: 11, ParentID: 1, Name: "财务部", Priority: 2},
		},
	}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          &transactionRunner{},
	})

	err := uc.UpdatePriorityDept(context.Background(), UpdatePriorityCommand{
		Items: []deptdomain.PriorityUpdate{{ID: 10, Priority: 1}},
	})
	if !errors.Is(err, deptdomain.ErrPriorityChanged) {
		t.Fatalf("UpdatePriorityDept() error = %v, want ErrPriorityChanged", err)
	}
}

func TestDeptUseCase_UpdatePriorityDeptRejectsInvalidPriority(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		depts: map[uint64]*deptdomain.Dept{
			10: {ID: 10, ParentID: 1, Name: "研发部", Priority: 1},
			11: {ID: 11, ParentID: 1, Name: "财务部", Priority: 2},
		},
	}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          &transactionRunner{},
	})

	err := uc.UpdatePriorityDept(context.Background(), UpdatePriorityCommand{
		Items: []deptdomain.PriorityUpdate{
			{ID: 10, Priority: 1},
			{ID: 11, Priority: 1},
		},
	})
	if !errors.Is(err, deptdomain.ErrInvalidPriority) {
		t.Fatalf("UpdatePriorityDept() error = %v, want ErrInvalidPriority", err)
	}
}

func TestDeptUseCase_DeleteDeptCascadeRejectsAssignedDept(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		depts: map[uint64]*deptdomain.Dept{
			10: {ID: 10, Code: "10", Name: "总部"},
			11: {ID: 11, Code: "10-11", ParentID: 10, Name: "研发部"},
		},
	}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          &transactionRunner{},
		Assignments:    &deptAssignments{count: 4},
	})

	err := uc.DeleteDeptCascade(context.Background(), 10)
	if !errors.Is(err, deptdomain.ErrInUse) {
		t.Fatalf("DeleteDeptCascade() error = %v, want ErrInUse", err)
	}
	var inUse deptdomain.InUseError
	if !errors.As(err, &inUse) || inUse.Count != 4 {
		t.Fatalf("DeleteDeptCascade() in-use error = %#v, want count 4", err)
	}
	if len(repo.depts) != 2 {
		t.Fatalf("dept count = %d, want unchanged 2", len(repo.depts))
	}
}

func TestDeptUseCase_DeleteDeptCascade(t *testing.T) {
	t.Parallel()

	repo := &deptRepo{
		depts: map[uint64]*deptdomain.Dept{
			10: {ID: 10, Code: "10", Name: "总部"},
			11: {ID: 11, Code: "10-11", ParentID: 10, Name: "研发部"},
			20: {ID: 20, Code: "20", Name: "分部"},
		},
	}
	tx := &transactionRunner{}
	locks := &deptLocks{}
	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: repo,
		Mysql:          tx,
		DeptLocks:      locks,
		Assignments:    &deptAssignments{},
	})

	if err := uc.DeleteDeptCascade(context.Background(), 10); err != nil {
		t.Fatalf("DeleteDeptCascade() error = %v", err)
	}
	if !tx.called {
		t.Fatal("DeleteDeptCascade() did not run in transaction")
	}
	if locks.deleteCalls != 1 {
		t.Fatalf("delete locks = %d, want 1", locks.deleteCalls)
	}
	if _, ok := repo.depts[10]; ok {
		t.Fatal("root dept was not deleted")
	}
	if _, ok := repo.depts[11]; ok {
		t.Fatal("child dept was not deleted")
	}
	if _, ok := repo.depts[20]; !ok {
		t.Fatal("unrelated dept was deleted")
	}
}

func TestDeptUseCase_CheckDeleteDept(t *testing.T) {
	t.Parallel()

	uc := NewDeptUseCase(DeptOptions{
		DeptRepository: &deptRepo{
			depts: map[uint64]*deptdomain.Dept{
				10: {ID: 10, Code: "10", Name: "总部"},
			},
		},
		Assignments: &deptAssignments{count: 2},
	})

	msg, err := uc.CheckDeleteDept(context.Background(), 10)
	if err != nil {
		t.Fatalf("CheckDeleteDept() error = %v", err)
	}
	want := "有2个与该组织关联的账号, 请先编辑账号所属组织后再进行删除"
	if msg != want {
		t.Fatalf("CheckDeleteDept() msg = %q, want %q", msg, want)
	}
}

type deptRepo struct {
	depts  map[uint64]*deptdomain.Dept
	nextID uint64
}

func (r *deptRepo) NextID(context.Context) (uint64, error) {
	if r.nextID == 0 {
		r.nextID = 1
	}
	id := r.nextID
	r.nextID++
	return id, nil
}

func (r *deptRepo) Create(_ context.Context, dept *deptdomain.Dept) error {
	if r.depts == nil {
		r.depts = map[uint64]*deptdomain.Dept{}
	}
	r.depts[dept.ID] = cloneDept(dept)
	return nil
}

func (r *deptRepo) UpdateName(_ context.Context, id uint64, name string) error {
	saved, ok := r.depts[id]
	if !ok {
		return deptdomain.ErrNotFound
	}
	saved.Name = name
	return nil
}

func (r *deptRepo) UpdatePriorities(_ context.Context, updates []deptdomain.PriorityUpdate) error {
	for _, update := range updates {
		saved, ok := r.depts[update.ID]
		if !ok {
			return deptdomain.ErrNotFound
		}
		saved.Priority = update.Priority
	}
	return nil
}

func (r *deptRepo) DeleteByIDs(_ context.Context, ids []uint64) error {
	for _, id := range ids {
		delete(r.depts, id)
	}
	return nil
}

func (r *deptRepo) FindByID(_ context.Context, id uint64) (*deptdomain.Dept, error) {
	dept, ok := r.depts[id]
	if !ok {
		return nil, deptdomain.ErrNotFound
	}
	return cloneDept(dept), nil
}

func (r *deptRepo) FindSubtree(_ context.Context, id uint64) ([]*deptdomain.Dept, error) {
	root, ok := r.depts[id]
	if !ok {
		return nil, nil
	}
	depts := make([]*deptdomain.Dept, 0)
	prefix := root.Code
	for _, dept := range r.depts {
		if dept.Code == prefix || hasDeptCodePrefix(dept.Code, prefix) {
			depts = append(depts, cloneDept(dept))
		}
	}
	return depts, nil
}

func (r *deptRepo) CountByParent(_ context.Context, parentID uint64) (int64, error) {
	var count int64
	for _, dept := range r.depts {
		if dept.ParentID == parentID {
			count++
		}
	}
	return count, nil
}

func (r *deptRepo) FindByParentAndName(_ context.Context, parentID uint64, name string) (*deptdomain.Dept, error) {
	for _, dept := range r.depts {
		if dept.ParentID == parentID && dept.Name == name {
			return cloneDept(dept), nil
		}
	}
	return nil, deptdomain.ErrNotFound
}

func cloneDept(dept *deptdomain.Dept) *deptdomain.Dept {
	if dept == nil {
		return nil
	}
	cp := *dept
	return &cp
}

func hasDeptCodePrefix(code string, prefix string) bool {
	return len(code) > len(prefix) && code[:len(prefix)+1] == prefix+"-"
}

type deptAssignments struct {
	count int64
}

func (a *deptAssignments) CountByDeptIDs(context.Context, []uint64) (int64, error) {
	return a.count, nil
}

type deptLocks struct {
	createCalls int
	updateCalls int
	deleteCalls int
}

func (l *deptLocks) WithDeptCreateLock(ctx context.Context, fn func(context.Context) error) error {
	l.createCalls++
	return fn(ctx)
}

func (l *deptLocks) WithDeptUpdateLocks(ctx context.Context, fn func(context.Context) error) error {
	l.updateCalls++
	return fn(ctx)
}

func (l *deptLocks) WithDeptDeleteLocks(ctx context.Context, fn func(context.Context) error) error {
	l.deleteCalls++
	return fn(ctx)
}
