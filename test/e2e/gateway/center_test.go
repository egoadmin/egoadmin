//go:build e2e

package main

import "testing"

func TestCenterProfileAndPassword(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	apis := e2e.apiIDMap(t, admin.Token)
	deptID := e2e.createDept(t, admin.Token, "0", e2e.uniqueName(t, "_center_dept"))
	roleID := e2e.createRole(t, admin.Token, e2e.uniqueName(t, "_center_role"), []string{"30201"}, requireAPIIDs(t, apis,
		"USER.V1.USERSERVICE/GETUSERLIST",
		"USER.V1.USERSERVICE/UGETDEPT",
		"USER.V1.ROLESERVICE/GETROLEALL",
	))
	username := trimUsername(e2e.uniqueName(t, "_center_user"))
	phone := uniquePhone(t)
	e2e.createUser(t, admin.Token, username, phone, deptID, []string{roleID})
	oldPassword := defaultPasswordFromPhone(phone)

	normal := e2e.loginAs(t, username, oldPassword)
	updatedName := username + "n"
	e2e.postJSON(t, "/api/user.v1.CenterService/EditCenterInfo", map[string]any{
		"name":   updatedName,
		"gender": 2,
	}, normal.Token, &emptyResponse{})

	var center centerInfoResponse
	e2e.postJSON(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, normal.Token, &center)
	if center.User.Name != updatedName {
		t.Fatalf("center name = %q, want %q", center.User.Name, updatedName)
	}

	newPhone := uniquePhone(t)
	phoneCrypto := e2e.buildCryptoRequest(t, username, "center.edit_info", map[string]any{
		"password": oldPassword,
	})
	phoneCrypto["name"] = updatedName
	phoneCrypto["gender"] = 2
	phoneCrypto["phone"] = newPhone
	e2e.postJSON(t, "/api/user.v1.CenterService/EditCenterInfo", phoneCrypto, normal.Token, &emptyResponse{})

	newPassword := "E2ePwd" + defaultPasswordFromPhone(newPhone)
	passCrypto := e2e.buildCryptoRequest(t, username, "center.edit_password", map[string]any{
		"oldPassword": oldPassword,
		"newPassword": newPassword,
	})
	e2e.postJSON(t, "/api/user.v1.CenterService/EditCenterPassword", passCrypto, normal.Token, &emptyResponse{})

	resp := e2e.postRaw(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, normal.Token)
	requireErrorResponse(t, resp, "GetCenterInfo after EditCenterPassword")

	resp = e2e.postRaw(t, "/api/user.v1.UserService/Login", e2e.buildLoginRequest(t, username, oldPassword, "login"), "")
	requireErrorResponse(t, resp, "Login with old password after EditCenterPassword")

	afterPassword := e2e.loginAs(t, username, newPassword)
	e2e.postJSON(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, afterPassword.Token, &center)
	if center.User.Phone != newPhone {
		t.Fatalf("center phone = %q, want %q", center.User.Phone, newPhone)
	}
}
