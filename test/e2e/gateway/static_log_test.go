//go:build e2e

package main

import (
	"strings"
	"testing"
)

func TestAuditLogAndStaticFallback(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	e2e.waitForLog(t, admin.Token, "admin", "登录")

	deptName := e2e.uniqueName(t, "_log_dept")
	deptID := e2e.createDept(t, admin.Token, "0", deptName)
	e2e.waitForLog(t, admin.Token, "admin", "新增组织")

	e2e.postJSON(t, "/api/user.v1.DeptService/DeleteDeptCascade", map[string]any{"id": deptID}, admin.Token, &emptyResponse{})
	e2e.waitForLog(t, admin.Token, "admin", "删除组织")

	index := string(e2e.requireGETStatus(t, "/e2e/non-existing/spa-route", 200))
	if !strings.Contains(strings.ToLower(index), "<!doctype html") {
		t.Fatalf("SPA fallback did not return index html: %q", firstBytes(index, 120))
	}
	if !strings.Contains(index, `src="/app-config.js"`) {
		t.Fatalf("SPA fallback did not load app-config.js: %q", firstBytes(index, 240))
	}

	appConfig := string(e2e.requireGETStatus(t, "/app-config.js", 200))
	if !strings.Contains(appConfig, "window.__APP_CONFIG__=") {
		t.Fatalf("app-config.js missing APP_CONFIG assignment: %q", firstBytes(appConfig, 240))
	}
	if !strings.Contains(appConfig, `"offlineOnPageLeave":false`) {
		t.Fatalf("runtime frontend config missing offlineOnPageLeave=false: %q", firstBytes(appConfig, 240))
	}

	apiResp := e2e.postRaw(t, "/api/user.v1.UserService/NoSuchMethod", map[string]any{}, admin.Token)
	requireErrorResponse(t, apiResp, "unknown /api method")
}

func firstBytes(value string, n int) string {
	if len(value) <= n {
		return value
	}
	return value[:n]
}
