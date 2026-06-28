//go:build e2e

package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDeptCRUD(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	topName := e2e.uniqueName(t, "_dept_top")
	childName := e2e.uniqueName(t, "_dept_child")

	topID := e2e.createDept(t, admin.Token, "0", topName)
	childID := e2e.createDept(t, admin.Token, topID, childName)

	var tops deptsResponse
	e2e.postJSON(t, "/api/user.v1.DeptService/GetDeptTop", map[string]any{}, admin.Token, &tops)
	if !containsDept(tops.Depts, topID) {
		t.Fatalf("GetDeptTop did not include top dept %s: %#v", topID, tops.Depts)
	}

	e2e.postJSON(t, "/api/user.v1.DeptService/UpdatePriorityDept", map[string]any{
		"priorities": []map[string]any{{"id": childID, "priority": 1}},
	}, admin.Token, &emptyResponse{})

	var top deptResponse
	e2e.postJSON(t, "/api/user.v1.DeptService/GetDept", map[string]any{"id": topID}, admin.Token, &top)
	if top.Dept.DeptName != topName {
		t.Fatalf("GetDept name = %q, want %q", top.Dept.DeptName, topName)
	}
	if !top.Dept.HasChild {
		t.Fatalf("GetDept(%s).hasChild = false, want true", topID)
	}

	var children deptsResponse
	e2e.postJSON(t, "/api/user.v1.DeptService/GetDeptChild", map[string]any{"id": topID}, admin.Token, &children)
	if !containsDept(children.Depts, childID) {
		t.Fatalf("GetDeptChild(%s) did not include child %s: %#v", topID, childID, children.Depts)
	}

	var byName deptsResponse
	e2e.postJSON(t, "/api/user.v1.DeptService/GetDeptByName", map[string]any{"name": childName}, admin.Token, &byName)
	if !containsDeptTree(byName.Depts, childID) {
		t.Fatalf("GetDeptByName(%q) did not include child %s: %#v", childName, childID, byName.Depts)
	}

	updatedChildName := childName + "_updated"
	e2e.postJSON(t, "/api/user.v1.DeptService/UpdateDept", map[string]any{
		"id":   childID,
		"dept": map[string]any{"deptName": updatedChildName},
	}, admin.Token, &emptyResponse{})

	var child deptResponse
	e2e.postJSON(t, "/api/user.v1.DeptService/GetDept", map[string]any{"id": childID}, admin.Token, &child)
	if child.Dept.DeptName != updatedChildName {
		t.Fatalf("updated child name = %q, want %q", child.Dept.DeptName, updatedChildName)
	}

	var check checkDeleteResponse
	e2e.postJSON(t, "/api/user.v1.DeptService/CheckDeptDelete", map[string]any{"id": topID}, admin.Token, &check)
	if !check.IsAllow {
		t.Fatalf("CheckDeptDelete(%s) isAllow = false, msg=%q", topID, check.Msg)
	}

	e2e.postJSON(t, "/api/user.v1.DeptService/DeleteDeptCascade", map[string]any{"id": topID}, admin.Token, &emptyResponse{})
	resp := e2e.postRaw(t, "/api/user.v1.DeptService/GetDept", map[string]any{"id": childID}, admin.Token)
	requireErrorResponse(t, resp, "GetDept after DeleteDeptCascade")
}

func TestRoleCRUDAndPermissionBoundary(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	apis := e2e.apiIDMap(t, admin.Token)
	roleName := e2e.uniqueName(t, "_role")
	roleAPIIDs := requireAPIIDs(t, apis,
		"USER.V1.ROLESERVICE/GETROLELIST",
		"USER.V1.ROLESERVICE/GETROLEALL",
		"USER.V1.USERSERVICE/GETAPIS",
		"USER.V1.ROLESERVICE/GETROLE",
		"USER.V1.ROLESERVICE/UPDATEROLE",
	)

	roleID := e2e.createRole(t, admin.Token, roleName, []string{"20101", "20103"}, roleAPIIDs)
	e2e.waitForLogMatching(t, admin.Token, "admin", "新增角色", roleName)
	deptID := e2e.createDept(t, admin.Token, "0", e2e.uniqueName(t, "_role_ref_dept"))
	refUserID := e2e.createUser(t, admin.Token, trimUsername(e2e.uniqueName(t, "_role_ref_user")), uniquePhone(t), deptID, []string{roleID})

	var got roleResponse
	e2e.postJSON(t, "/api/user.v1.RoleService/GetRole", map[string]any{"id": roleID}, admin.Token, &got)
	if got.Role.Name != roleName {
		t.Fatalf("GetRole name = %q, want %q", got.Role.Name, roleName)
	}
	if !containsString(got.Role.Apis, apis["USER.V1.ROLESERVICE/UPDATEROLE"]) {
		t.Fatalf("GetRole apis missing UpdateRole id %s: %#v", apis["USER.V1.ROLESERVICE/UPDATEROLE"], got.Role.Apis)
	}

	var list roleListResponse
	e2e.postJSON(t, "/api/user.v1.RoleService/GetRoleList", map[string]any{
		"page":  1,
		"limit": 10,
		"name":  roleName,
	}, admin.Token, &list)
	if list.Total < 1 || !containsRole(list.Roles, roleID) {
		t.Fatalf("GetRoleList did not include role %s: total=%d roles=%#v", roleID, list.Total, list.Roles)
	}

	var all roleAllResponse
	e2e.postJSON(t, "/api/user.v1.RoleService/GetRoleAll", map[string]any{}, admin.Token, &all)
	if !containsRole(all.Roles, roleID) {
		t.Fatalf("GetRoleAll did not include role %s", roleID)
	}

	resp := e2e.postRaw(t, "/api/user.v1.RoleService/AddRole", map[string]any{
		"role": rolePayload(e2e.uniqueName(t, "_bad_role"), []string{"20101"}, requireAPIIDs(t, apis, "USER.V1.USERSERVICE/ADDUSER")),
	}, admin.Token)
	requireErrorResponse(t, resp, "AddRole with API outside selected viewMenus")

	updatedName := roleName + "_updated"
	e2e.postJSON(t, "/api/user.v1.RoleService/UpdateRole", map[string]any{
		"id":   roleID,
		"role": rolePayload(updatedName, []string{"20101"}, requireAPIIDs(t, apis, "USER.V1.ROLESERVICE/GETROLELIST")),
	}, admin.Token, &emptyResponse{})
	e2e.waitForLogMatching(t, admin.Token, "admin", "编辑角色", updatedName)

	var updated roleResponse
	e2e.postJSON(t, "/api/user.v1.RoleService/GetRole", map[string]any{"id": roleID}, admin.Token, &updated)
	if updated.Role.Name != updatedName {
		t.Fatalf("updated role name = %q, want %q", updated.Role.Name, updatedName)
	}

	var check checkDeleteResponse
	e2e.postJSON(t, "/api/user.v1.RoleService/CheckDeleteRole", map[string]any{"id": roleID}, admin.Token, &check)
	if check.IsAllow || check.Msg == "" {
		t.Fatalf("CheckDeleteRole(%s) = allow=%v msg=%q, want referenced-role rejection", roleID, check.IsAllow, check.Msg)
	}
	resp = e2e.postRaw(t, "/api/user.v1.RoleService/DeleteRole", map[string]any{"id": roleID}, admin.Token)
	requireErrorResponse(t, resp, "DeleteRole referenced by user")

	e2e.postJSON(t, "/api/user.v1.UserService/DeleteUser", map[string]any{"ids": []string{refUserID}}, admin.Token, &emptyResponse{})
	e2e.postJSON(t, "/api/user.v1.RoleService/CheckDeleteRole", map[string]any{"id": roleID}, admin.Token, &check)
	if !check.IsAllow {
		t.Fatalf("CheckDeleteRole(%s) isAllow = false, msg=%q", roleID, check.Msg)
	}
	e2e.postJSON(t, "/api/user.v1.RoleService/DeleteRole", map[string]any{"id": roleID}, admin.Token, &emptyResponse{})
	e2e.waitForLogMatching(t, admin.Token, "admin", "删除角色", roleID)
	resp = e2e.postRaw(t, "/api/user.v1.RoleService/GetRole", map[string]any{"id": roleID}, admin.Token)
	requireErrorResponse(t, resp, "GetRole after DeleteRole")
}

func TestRoleDataPermissionOutOfScopeReturnsAccessDenied(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	apis := e2e.apiIDMap(t, admin.Token)
	roleAPIs := requireAPIIDs(t, apis,
		"USER.V1.ROLESERVICE/GETROLELIST",
		"USER.V1.ROLESERVICE/GETROLEALL",
		"USER.V1.USERSERVICE/GETAPIS",
		"USER.V1.ROLESERVICE/ADDROLE",
	)
	userAPIs := requireAPIIDs(t, apis,
		"USER.V1.USERSERVICE/GETUSERLIST",
		"USER.V1.USERSERVICE/UGETDEPT",
		"USER.V1.ROLESERVICE/GETROLEALL",
	)

	var addRole idResponse
	e2e.postJSON(t, "/api/user.v1.RoleService/AddRole", map[string]any{
		"role": rolePayloadWithDataPerm(e2e.uniqueName(t, "_limited_role"), []string{"20101", "20102", "30201"}, append(roleAPIs, userAPIs...), 3),
	}, admin.Token, &addRole)
	if addRole.ID == "" || addRole.ID == "0" {
		t.Fatalf("AddRole returned invalid id %q", addRole.ID)
	}

	deptID := e2e.createDept(t, admin.Token, "0", e2e.uniqueName(t, "_limited_dept"))
	username := trimUsername(e2e.uniqueName(t, "_limited_user"))
	phone := uniquePhone(t)
	e2e.createUser(t, admin.Token, username, phone, deptID, []string{addRole.ID})

	limited := e2e.loginAs(t, username, defaultPasswordFromPhone(phone))
	resp := e2e.postRaw(t, "/api/user.v1.RoleService/AddRole", map[string]any{
		"role": rolePayload(e2e.uniqueName(t, "_out_scope_role"), []string{"20101", "20102"}, roleAPIs),
	}, limited.Token)
	errResp := requireEgoErrorReason(t, resp, "AddRole data permission out of scope", "ecode.v1.ERROR_ACCESS_DENIED")
	if errResp.Message == "" || strings.Contains(errResp.Message, "内部错误") {
		t.Fatalf("AddRole data permission out of scope message = %q, want user-facing access denied message", errResp.Message)
	}
}

func TestUserCRUDPermissionAndForcedOffline(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	apis := e2e.apiIDMap(t, admin.Token)

	deptID := e2e.createDept(t, admin.Token, "0", e2e.uniqueName(t, "_user_dept"))
	roleID := e2e.createRole(t, admin.Token, e2e.uniqueName(t, "_user_role"), []string{"30201", "30203"}, requireAPIIDs(t, apis,
		"USER.V1.USERSERVICE/GETUSERLIST",
		"USER.V1.USERSERVICE/UGETDEPT",
		"USER.V1.ROLESERVICE/GETROLEALL",
		"USER.V1.USERSERVICE/GETUSER",
		"USER.V1.USERSERVICE/UPDATEUSER",
	))

	username := trimUsername(e2e.uniqueName(t, ""))
	phone := uniquePhone(t)
	userID := e2e.createUser(t, admin.Token, username, phone, deptID, []string{roleID})
	e2e.waitForLogMatching(t, admin.Token, "admin", "新增用户", username)

	var got userResponse
	e2e.postJSON(t, "/api/user.v1.UserService/GetUser", map[string]any{"id": userID}, admin.Token, &got)
	if got.User.Username != username {
		t.Fatalf("GetUser username = %q, want %q", got.User.Username, username)
	}
	if got.User.Password != "" {
		t.Fatalf("GetUser exposed password field: %q", got.User.Password)
	}

	var list userListResponse
	e2e.postJSON(t, "/api/user.v1.UserService/GetUserList", map[string]any{
		"page":       1,
		"limit":      10,
		"name":       username,
		"userStatus": -1,
	}, admin.Token, &list)
	if list.Total < 1 || !containsUser(list.Users, userID) {
		t.Fatalf("GetUserList did not include user %s: total=%d users=%#v", userID, list.Total, list.Users)
	}

	updatedName := username + "_n"
	updatedPhone := uniquePhone(t)
	e2e.postJSON(t, "/api/user.v1.UserService/UpdateUser", map[string]any{
		"id":         userID,
		"name":       updatedName,
		"gender":     2,
		"roleIds":    []string{roleID},
		"deptId":     deptID,
		"phone":      updatedPhone,
		"userStatus": 1,
	}, admin.Token, &emptyResponse{})
	e2e.waitForLogMatching(t, admin.Token, "admin", "编辑用户", updatedName)

	normal := e2e.loginAs(t, username, defaultPasswordFromPhone(phone))
	var selfCenter centerInfoResponse
	e2e.postJSON(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, normal.Token, &selfCenter)
	if selfCenter.User.Username != username {
		t.Fatalf("normal user center username = %q, want %q", selfCenter.User.Username, username)
	}

	resp := e2e.postRaw(t, "/api/user.v1.UserService/AddUser", map[string]any{
		"roleIds": []string{roleID},
		"user": map[string]any{
			"username":   trimUsername(e2e.uniqueName(t, "_forbidden")),
			"name":       "forbidden",
			"phone":      uniquePhone(t),
			"gender":     1,
			"userStatus": 1,
			"deptId":     deptID,
		},
	}, normal.Token)
	requireErrorResponse(t, resp, "non-admin AddUser without permission")

	resp = e2e.postRaw(t, "/api/user.v1.DeptService/DeleteDeptCascade", map[string]any{"id": deptID}, admin.Token)
	requireErrorResponse(t, resp, "DeleteDeptCascade referenced by user")

	e2e.postJSON(t, "/api/user.v1.UserService/HeartBeatUser", map[string]any{}, normal.Token, &emptyResponse{})
	var online userListResponse
	e2e.postJSON(t, "/api/user.v1.UserService/GetOnlineUserList", map[string]any{
		"page":  1,
		"limit": 20,
		"name":  username,
	}, admin.Token, &online)
	if !containsUser(online.Users, userID) {
		t.Fatalf("GetOnlineUserList did not include user %s: %#v", userID, online.Users)
	}

	e2e.postJSON(t, "/api/user.v1.UserService/OfflineUser", map[string]any{"ids": []string{userID}}, admin.Token, &emptyResponse{})
	e2e.waitForLogMatching(t, admin.Token, "admin", "强退用户", userID)
	resp = e2e.postRaw(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, normal.Token)
	requireErrorResponse(t, resp, "GetCenterInfo after OfflineUser")

	e2e.postJSON(t, "/api/user.v1.UserService/ResetUserPassword", map[string]any{"id": userID}, admin.Token, &emptyResponse{})
	e2e.waitForLogMatching(t, admin.Token, "admin", "重置密码", userID)
	resetLogin := e2e.loginAs(t, username, defaultPasswordFromPhone(updatedPhone))
	e2e.postJSON(t, "/api/user.v1.UserService/DeleteUser", map[string]any{"ids": []string{userID}}, admin.Token, &emptyResponse{})
	e2e.waitForLogMatching(t, admin.Token, "admin", "删除用户", userID)
	resp = e2e.postRaw(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, resetLogin.Token)
	requireErrorResponse(t, resp, "GetCenterInfo after DeleteUser")

	resp = e2e.postRaw(t, "/api/user.v1.UserService/DeleteUser", map[string]any{"ids": []string{adminID(t, admin.Token)}}, admin.Token)
	requireErrorResponse(t, resp, "DeleteUser built-in admin")
}

func TestHeartbeatTimeoutOnlyMarksOfflineByDefault(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	deptID := e2e.createDept(t, admin.Token, "0", e2e.uniqueName(t, "_heartbeat_dept"))
	roleID := e2e.createRole(t, admin.Token, e2e.uniqueName(t, "_heartbeat_role"), []string{"30201"}, []string{})
	username := trimUsername(e2e.uniqueName(t, "_heartbeat_user"))
	phone := uniquePhone(t)
	userID := e2e.createUser(t, admin.Token, username, phone, deptID, []string{roleID})

	normal := e2e.loginAs(t, username, defaultPasswordFromPhone(phone))
	e2e.postJSON(t, "/api/user.v1.UserService/HeartBeatUser", map[string]any{}, normal.Token, &emptyResponse{})

	db := e2e.openUserDB(t)
	defer db.Close()
	if _, err := db.ExecContext(t.Context(), "UPDATE user SET heartbeat_time = DATE_SUB(NOW(), INTERVAL 20 MINUTE), user_online = 1 WHERE id = ?", userID); err != nil {
		t.Fatalf("expire heartbeat for user %s: %v", userID, err)
	}

	deadline := time.Now().Add(25 * time.Second)
	var online userListResponse
	for time.Now().Before(deadline) {
		e2e.postJSON(t, "/api/user.v1.UserService/GetOnlineUserList", map[string]any{
			"page":  1,
			"limit": 20,
			"name":  username,
		}, admin.Token, &online)
		if !containsUser(online.Users, userID) {
			var selfCenter centerInfoResponse
			e2e.postJSON(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, normal.Token, &selfCenter)
			if selfCenter.User.Username != username {
				t.Fatalf("center username = %q, want %q", selfCenter.User.Username, username)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("user %s remained online after heartbeat expiry; last online users: %#v", userID, online.Users)
}

func TestRoleUpdateChangesOrdinaryUserPermission(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	apis := e2e.apiIDMap(t, admin.Token)
	deptID := e2e.createDept(t, admin.Token, "0", e2e.uniqueName(t, "_role_update_dept"))
	roleID := e2e.createRole(t, admin.Token, e2e.uniqueName(t, "_role_update_role"), []string{"30201", "30203"}, requireAPIIDs(t, apis,
		"USER.V1.USERSERVICE/GETUSERLIST",
		"USER.V1.USERSERVICE/UGETDEPT",
		"USER.V1.ROLESERVICE/GETROLEALL",
		"USER.V1.USERSERVICE/GETUSER",
		"USER.V1.USERSERVICE/UPDATEUSER",
	))
	username := trimUsername(e2e.uniqueName(t, "_role_update_user"))
	phone := uniquePhone(t)
	userID := e2e.createUser(t, admin.Token, username, phone, deptID, []string{roleID})

	normal := e2e.loginAs(t, username, defaultPasswordFromPhone(phone))
	var got userResponse
	e2e.postJSON(t, "/api/user.v1.UserService/GetUser", map[string]any{"id": userID}, normal.Token, &got)
	if got.User.ID != userID {
		t.Fatalf("ordinary user GetUser id = %q, want %q", got.User.ID, userID)
	}

	e2e.postJSON(t, "/api/user.v1.RoleService/UpdateRole", map[string]any{
		"id": roleID,
		"role": rolePayload(e2e.uniqueName(t, "_role_update_new"), []string{"30201"}, requireAPIIDs(t, apis,
			"USER.V1.USERSERVICE/GETUSERLIST",
			"USER.V1.USERSERVICE/UGETDEPT",
			"USER.V1.ROLESERVICE/GETROLEALL",
		)),
	}, admin.Token, &emptyResponse{})

	resp := e2e.postRaw(t, "/api/user.v1.UserService/GetUser", map[string]any{"id": userID}, normal.Token)
	requireErrorResponse(t, resp, "ordinary user GetUser after role permission removed")
}

func containsDept(depts []deptInfo, id string) bool {
	for _, dept := range depts {
		if dept.ID == id {
			return true
		}
	}
	return false
}

func containsDeptTree(depts []deptInfo, id string) bool {
	for _, dept := range depts {
		if dept.ID == id || containsDeptTree(dept.Childs, id) {
			return true
		}
	}
	return false
}

func containsRole(roles []roleInfo, id string) bool {
	for _, role := range roles {
		if role.ID == id {
			return true
		}
	}
	return false
}

func containsUser(users []userInfo, id string) bool {
	for _, user := range users {
		if user.ID == id {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func trimUsername(value string) string {
	value = strings.NewReplacer("_", "", "-", "", "/", "").Replace(value)
	if len(value) > 16 {
		value = value[len(value)-16:]
	}
	return "u" + value
}

func uniquePhone(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("139%08d", timeNowNanoMod()%100000000)
}

func timeNowNanoMod() int64 {
	return time.Now().UnixNano() % 100000000
}

func adminID(t *testing.T, token string) string {
	t.Helper()
	var center centerInfoResponse
	e2e.postJSON(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, token, &center)
	if center.User.ID == "" {
		t.Fatal("admin center returned empty user id")
	}
	return center.User.ID
}
