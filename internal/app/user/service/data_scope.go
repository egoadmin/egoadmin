package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	store "github.com/egoadmin/egoadmin/internal/app/user/internal/store"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	componentjetcache "github.com/egoadmin/egoadmin/internal/component/jetcache"
	"github.com/egoadmin/egoadmin/internal/platform/defaults"
	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	stdjetcache "github.com/mgtv-tech/jetcache-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const dataScopeTTL = 2 * time.Minute

type DataScopeLevel int32

const (
	DataScopeAll        DataScopeLevel = DataScopeLevel(store.RoleModelDataPermAll)
	DataScopeDeptAndSub DataScopeLevel = DataScopeLevel(store.RoleModelDataPermUserDeptAndSubDept)
	DataScopeDeptSelf   DataScopeLevel = DataScopeLevel(store.RoleModelDataPermUserDeptSelf)
	DataScopeSelf       DataScopeLevel = DataScopeLevel(store.RoleModelUserSelf)
)

type DataScope struct {
	UserID  uint64         `json:"userID"`
	DeptID  uint64         `json:"deptID"`
	Level   DataScopeLevel `json:"level"`
	DeptIDs []uint64       `json:"deptIDs"`
	IsAdmin bool           `json:"isAdmin"`
}

type dataScopeCache interface {
	Get(ctx context.Context, key string, val *DataScope) error
	Set(ctx context.Context, key string, val DataScope, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

func (s *UserService) DataScope(ctx context.Context) (DataScope, error) {
	return s.resolveDataScope(ctx)
}

func (s *LogService) DataScope(ctx context.Context) (DataScope, error) {
	return resolveDataScope(ctx, s.User, s.Role, s.Dept, newJetDataScopeCache(s.JetCache))
}

func (s *RoleService) DataScope(ctx context.Context) (DataScope, error) {
	return resolveDataScope(ctx, s.User, s.Role, s.Dept, newJetDataScopeCache(s.JetCache))
}

func (s *DeptService) DataScope(ctx context.Context) (DataScope, error) {
	return resolveDataScope(ctx, s.User, s.Role, s.Dept, newJetDataScopeCache(s.JetCache))
}

func (s *UserService) resolveDataScope(ctx context.Context) (DataScope, error) {
	return resolveDataScope(ctx, s.User, s.Role, s.Dept, newJetDataScopeCache(s.JetCache))
}

func resolveDataScope(ctx context.Context, user store.UserInterface, role store.RoleInterface, dept store.DeptInterface, cache dataScopeCache) (DataScope, error) {
	auth, ok := authsession.FromContext(ctx)
	if !ok || auth.UserID == 0 {
		return DataScope{}, platformi18n.ErrorNotLogin(ctx, "AuthMissingToken", nil)
	}
	if auth.IsBuiltinAdmin {
		return DataScope{
			UserID:  auth.UserID,
			DeptID:  auth.DeptID,
			Level:   DataScopeAll,
			IsAdmin: true,
		}, nil
	}

	key := dataScopeCacheKey(auth.UserID)
	if cache != nil {
		var cached DataScope
		if err := cache.Get(ctx, key, &cached); err == nil && cached.UserID == auth.UserID {
			return cached, nil
		} else if err != nil && !errors.Is(err, stdjetcache.ErrCacheMiss) && !errors.Is(err, gorm.ErrRecordNotFound) {
			return DataScope{}, err
		}
	}

	scope, err := loadDataScope(ctx, auth, user, dept)
	if err != nil {
		return DataScope{}, err
	}
	if cache != nil {
		if err = cache.Set(ctx, key, scope, dataScopeTTL); err != nil {
			return DataScope{}, err
		}
	}
	return scope, nil
}

func loadDataScope(ctx context.Context, auth *authsession.AuthContext, user store.UserInterface, dept store.DeptInterface) (DataScope, error) {
	savedUser, err := user.Get(ctx, auth.UserID)
	if err != nil {
		return DataScope{}, err
	}
	if savedUser.DeptID == 0 {
		return DataScope{}, platformi18n.ErrorAccessDenied(ctx, "CurrentUserNoDept", nil)
	}
	level := widestDataScope(savedUser.Roles)
	scope := DataScope{
		UserID: auth.UserID,
		DeptID: savedUser.DeptID,
		Level:  level,
	}

	switch level {
	case DataScopeAll:
		return scope, nil
	case DataScopeDeptAndSub:
		ids, er := dept.GetSubtreeIDs(ctx, savedUser.DeptID)
		if er != nil {
			return DataScope{}, er
		}
		scope.DeptIDs = ids
	case DataScopeDeptSelf:
		scope.DeptIDs = []uint64{savedUser.DeptID}
	case DataScopeSelf:
		scope.DeptIDs = []uint64{savedUser.DeptID}
	default:
		scope.Level = DataScopeSelf
		scope.DeptIDs = []uint64{savedUser.DeptID}
	}

	return scope, nil
}

func widestDataScope(roles []store.RoleModel) DataScopeLevel {
	if len(roles) == 0 {
		return DataScopeSelf
	}
	best := DataScopeSelf
	for _, role := range roles {
		level := DataScopeLevel(role.DataPerm)
		switch level {
		case DataScopeAll:
			return DataScopeAll
		case DataScopeDeptAndSub:
			if best == DataScopeDeptSelf || best == DataScopeSelf {
				best = DataScopeDeptAndSub
			}
		case DataScopeDeptSelf:
			if best == DataScopeSelf {
				best = DataScopeDeptSelf
			}
		case DataScopeSelf:
		default:
		}
	}
	return best
}

func (s DataScope) AllowsUser(user *store.UserModel) bool {
	if s.IsAdmin || s.Level == DataScopeAll {
		return true
	}
	if user == nil {
		return false
	}
	switch s.Level {
	case DataScopeDeptAndSub, DataScopeDeptSelf:
		return slices.Contains(s.DeptIDs, user.DeptID)
	case DataScopeSelf:
		return user.ID == s.UserID
	default:
		return false
	}
}

func (s DataScope) AllowsDeptID(deptID uint64) bool {
	if s.IsAdmin || s.Level == DataScopeAll {
		return true
	}
	if deptID == 0 {
		return false
	}
	return slices.Contains(s.DeptIDs, deptID)
}

func (s DataScope) AllowsRole(role *store.RoleModel) bool {
	if s.IsAdmin {
		return true
	}
	if role == nil {
		return false
	}
	if role.BuiltIn == store.RoleModelBuiltIn {
		return false
	}
	if !dataPermAssignable(s.Level, DataScopeLevel(role.DataPerm)) {
		return false
	}
	if role.OwnerUserID == s.UserID {
		return true
	}
	switch s.Level {
	case DataScopeAll:
		return role.OwnerUserID != 0 || role.OwnerDeptID != 0
	case DataScopeDeptAndSub, DataScopeDeptSelf:
		return role.OwnerDeptID != 0 && s.AllowsDeptID(role.OwnerDeptID)
	case DataScopeSelf:
		return false
	default:
		return false
	}
}

func (s DataScope) AllowsRoleMutable(role *store.RoleModel) bool {
	if s.IsAdmin {
		return true
	}
	if role == nil {
		return false
	}
	if role.BuiltIn == store.RoleModelBuiltIn {
		return false
	}
	if !dataPermAssignable(s.Level, DataScopeLevel(role.DataPerm)) {
		return false
	}
	if role.OwnerUserID == s.UserID {
		return true
	}
	switch s.Level {
	case DataScopeAll:
		return role.OwnerUserID != 0 || role.OwnerDeptID != 0
	case DataScopeDeptAndSub, DataScopeDeptSelf:
		return role.OwnerDeptID != 0 && s.AllowsDeptID(role.OwnerDeptID)
	case DataScopeSelf:
		return false
	default:
		return false
	}
}

func (s DataScope) UserScope() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.IsAdmin || s.Level == DataScopeAll {
			return db
		}
		switch s.Level {
		case DataScopeDeptAndSub, DataScopeDeptSelf:
			if len(s.DeptIDs) == 0 {
				return dataScopeDeny(db)
			}
			return db.Where(uint64ColumnIn("dept_id", s.DeptIDs))
		case DataScopeSelf:
			return db.Where(clause.Eq{
				Column: clause.Column{Table: clause.CurrentTable, Name: "id"},
				Value:  s.UserID,
			})
		default:
			return dataScopeDeny(db)
		}
	}
}

func (s DataScope) LogScope() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.IsAdmin || s.Level == DataScopeAll {
			return db
		}
		switch s.Level {
		case DataScopeDeptAndSub, DataScopeDeptSelf:
			if len(s.DeptIDs) == 0 {
				return dataScopeDeny(db)
			}
			return db.Where(uint64ColumnIn("dept_id_u64", s.DeptIDs))
		case DataScopeSelf:
			return db.Where(clause.Eq{
				Column: clause.Column{Table: clause.CurrentTable, Name: "user_id_u64"},
				Value:  s.UserID,
			})
		default:
			return dataScopeDeny(db)
		}
	}
}

func (s DataScope) RoleScope() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.IsAdmin {
			return db
		}
		if len(s.DeptIDs) == 0 && s.UserID == 0 {
			return dataScopeDeny(db)
		}
		owner := make([]clause.Expression, 0, 3)
		switch s.Level {
		case DataScopeAll:
			owner = append(owner,
				clause.Neq{Column: clause.Column{Table: clause.CurrentTable, Name: "owner_user_id"}, Value: uint64(0)},
				clause.Neq{Column: clause.Column{Table: clause.CurrentTable, Name: "owner_dept_id"}, Value: uint64(0)},
			)
		case DataScopeDeptAndSub, DataScopeDeptSelf:
			if s.UserID != 0 {
				owner = append(owner, clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: "owner_user_id"}, Value: s.UserID})
			}
			if len(s.DeptIDs) != 0 {
				owner = append(owner, uint64ColumnIn("owner_dept_id", s.DeptIDs))
			}
		case DataScopeSelf:
			owner = append(owner, clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: "owner_user_id"}, Value: s.UserID})
		default:
			return dataScopeDeny(db)
		}
		return db.Where(clause.AndConditions{Exprs: []clause.Expression{
			clause.Neq{Column: clause.Column{Table: clause.CurrentTable, Name: "built_in"}, Value: store.RoleModelBuiltIn},
			int32ColumnIn("data_perm", []int32{
				int32(DataScopeAll),
				int32(DataScopeDeptAndSub),
				int32(DataScopeDeptSelf),
				int32(DataScopeSelf),
			}),
			clause.Gte{Column: clause.Column{Table: clause.CurrentTable, Name: "data_perm"}, Value: int32(s.Level)},
			clause.OrConditions{Exprs: owner},
		}})
	}
}

func (s DataScope) DataPermAssignable(target DataScopeLevel) bool {
	if s.IsAdmin || s.Level == DataScopeAll {
		return validDataScopeLevel(target)
	}
	return dataPermAssignable(s.Level, target)
}

func dataPermAssignable(actor DataScopeLevel, target DataScopeLevel) bool {
	if !validDataScopeLevel(target) {
		return false
	}
	return int32(target) >= int32(actor)
}

func validDataScopeLevel(level DataScopeLevel) bool {
	switch level {
	case DataScopeAll, DataScopeDeptAndSub, DataScopeDeptSelf, DataScopeSelf:
		return true
	default:
		return false
	}
}

func (s DataScope) FilterDeptTree(depts []store.DeptModel) []store.DeptModel {
	if s.IsAdmin || s.Level == DataScopeAll {
		return depts
	}
	ids := make(map[uint64]struct{}, len(s.DeptIDs))
	for _, id := range s.DeptIDs {
		ids[id] = struct{}{}
	}
	if s.Level == DataScopeSelf && s.DeptID != 0 {
		ids[s.DeptID] = struct{}{}
	}
	return deptFilterByIDs(depts, ids)
}

func (s DataScope) FilterDeptModels(depts []*store.DeptModel) []*store.DeptModel {
	if s.IsAdmin || s.Level == DataScopeAll {
		return depts
	}
	out := make([]*store.DeptModel, 0, len(depts))
	for _, dept := range depts {
		if dept != nil && s.AllowsDeptID(dept.ID) {
			out = append(out, dept)
		}
	}
	return out
}

func (s DataScope) AllowsDeptMutableID(id uint64) bool {
	if s.IsAdmin || s.Level == DataScopeAll {
		return true
	}
	if s.Level == DataScopeSelf {
		return false
	}
	return s.AllowsDeptID(id)
}

func (s DataScope) EnforceUser(ctx context.Context, user *store.UserModel) error {
	if s.AllowsUser(user) {
		return nil
	}
	return platformi18n.ErrorAccessDenied(ctx, "NoAccessUserData", nil)
}

func (s DataScope) EnforceDeptID(ctx context.Context, id uint64) error {
	if s.AllowsDeptID(id) {
		return nil
	}
	return platformi18n.ErrorAccessDenied(ctx, "NoAccessDeptData", nil)
}

func (s DataScope) EnforceDeptMutableID(ctx context.Context, id uint64) error {
	if s.AllowsDeptMutableID(id) {
		return nil
	}
	return platformi18n.ErrorAccessDenied(ctx, "NoModifyDeptData", nil)
}

func (s DataScope) EnforceRole(ctx context.Context, role *store.RoleModel) error {
	if s.AllowsRole(role) {
		return nil
	}
	return platformi18n.ErrorAccessDenied(ctx, "NoAccessRoleData", nil)
}

func (s DataScope) EnforceRoleMutable(ctx context.Context, role *store.RoleModel) error {
	if s.AllowsRoleMutable(role) {
		return nil
	}
	return platformi18n.ErrorAccessDenied(ctx, "NoModifyRoleData", nil)
}

func (s DataScope) EnforceAssignableDataPerm(ctx context.Context, level int32) error {
	if s.DataPermAssignable(DataScopeLevel(level)) {
		return nil
	}
	return platformi18n.ErrorAccessDenied(ctx, "DataPermissionOutOfScope", nil)
}

// 预留:从部门模型列表提取去重后的部门 ID，供后续数据权限计算使用。
//
//nolint:unused // 预留:部门 ID 提取与去重
func deptIDsFromStore(depts []*store.DeptModel) []uint64 {
	ids := make([]uint64, 0, len(depts))
	seen := make(map[uint64]struct{}, len(depts))
	for _, dept := range depts {
		if dept == nil || dept.ID == 0 {
			continue
		}
		if _, ok := seen[dept.ID]; ok {
			continue
		}
		seen[dept.ID] = struct{}{}
		ids = append(ids, dept.ID)
	}
	return ids
}

func uint64SliceToInterfaces(ids []uint64) []interface{} {
	values := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		values = append(values, id)
	}
	return values
}

func uint64ColumnIn(name string, ids []uint64) clause.IN {
	return clause.IN{
		Column: clause.Column{Table: clause.CurrentTable, Name: name},
		Values: uint64SliceToInterfaces(ids),
	}
}

func int32ColumnIn(name string, values []int32) clause.IN {
	items := make([]interface{}, 0, len(values))
	for _, v := range values {
		items = append(items, v)
	}
	return clause.IN{
		Column: clause.Column{Table: clause.CurrentTable, Name: name},
		Values: items,
	}
}

func dataScopeDeny(db *gorm.DB) *gorm.DB {
	return db.Where(clause.Eq{
		Column: clause.Column{Table: clause.CurrentTable, Name: "id"},
		Value:  uint64(0),
	})
}

func dataScopeCacheKey(userID uint64) string {
	return fmt.Sprintf("%s:auth:data_scope:%d", defaults.RedisKeyPrefix, userID)
}

func deleteDataScopeCache(ctx context.Context, cache dataScopeCache, userIDs ...uint64) error {
	if cache == nil {
		return nil
	}
	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}
		if err := cache.Delete(ctx, dataScopeCacheKey(userID)); err != nil {
			return err
		}
	}
	return nil
}

func (s *UserService) DataScopeCache() dataScopeCache {
	if s == nil {
		return nil
	}
	return newJetDataScopeCache(s.JetCache)
}

func (s *RoleService) DataScopeCache() dataScopeCache {
	if s == nil {
		return nil
	}
	return newJetDataScopeCache(s.JetCache)
}

func (s *DeptService) DataScopeCache() dataScopeCache {
	if s == nil {
		return nil
	}
	return newJetDataScopeCache(s.JetCache)
}

func newJetDataScopeCache(component *componentjetcache.Component) dataScopeCache {
	if component == nil || component.Cache() == nil {
		return nil
	}
	return jetDataScopeCache{cache: component.Cache()}
}

type jetDataScopeCache struct {
	cache stdjetcache.Cache
}

func (c jetDataScopeCache) Get(ctx context.Context, key string, val *DataScope) error {
	return c.cache.Get(ctx, key, val)
}

func (c jetDataScopeCache) Set(ctx context.Context, key string, val DataScope, ttl time.Duration) error {
	return c.cache.Set(ctx, key, stdjetcache.Value(val), stdjetcache.TTL(ttl))
}

func (c jetDataScopeCache) Delete(ctx context.Context, key string) error {
	return c.cache.Delete(ctx, key)
}
