//go:build e2e

package main

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func (e *environment) uniqueName(t *testing.T, suffix string) string {
	t.Helper()

	name := strings.ToLower(t.Name())
	replacer := strings.NewReplacer("/", "_", " ", "_", "-", "_")
	name = replacer.Replace(name)
	if len(name) > 24 {
		name = name[:24]
	}
	run := e.runID
	if len(run) > 8 {
		run = run[len(run)-8:]
	}
	return fmt.Sprintf("e2e_%s_%s_%d", run, name, time.Now().UnixNano()%1_000_000_000_000) + suffix
}

func (e *environment) apiIDMap(t *testing.T, token string) map[string]string {
	t.Helper()

	var got apisResponse
	e.postJSON(t, "/api/user.v1.UserService/GetAPIs", map[string]any{}, token, &got)
	apis := make(map[string]string, len(got.Apis))
	for _, api := range got.Apis {
		apis[strings.ToUpper(api.FullPath)] = api.ID
	}
	return apis
}

func requireAPIIDs(t *testing.T, apis map[string]string, fullPaths ...string) []string {
	t.Helper()

	ids := make([]string, 0, len(fullPaths))
	for _, path := range fullPaths {
		id, ok := apis[strings.ToUpper(path)]
		if !ok {
			t.Fatalf("API catalog missing %s", path)
		}
		ids = append(ids, id)
	}
	return ids
}

func (e *environment) createDept(t *testing.T, token string, parentID string, name string) string {
	t.Helper()

	if parentID == "" {
		parentID = "0"
	}
	var out idResponse
	e.postJSON(t, "/api/user.v1.DeptService/AddDept", map[string]any{
		"parentId": parentID,
		"dept": map[string]any{
			"deptName": name,
			"leader":   "E2E",
			"phone":    "13800000000",
			"email":    "e2e@example.com",
			"remark":   "created by gateway e2e",
		},
	}, token, &out)
	if out.ID == "" || out.ID == "0" {
		t.Fatalf("AddDept returned invalid id %q", out.ID)
	}
	return out.ID
}

func (e *environment) createRole(t *testing.T, token string, name string, viewMenus []string, apiIDs []string) string {
	t.Helper()

	var out idResponse
	e.postJSON(t, "/api/user.v1.RoleService/AddRole", map[string]any{
		"role": rolePayload(name, viewMenus, apiIDs),
	}, token, &out)
	if out.ID == "" || out.ID == "0" {
		t.Fatalf("AddRole returned invalid id %q", out.ID)
	}
	return out.ID
}

func rolePayload(name string, viewMenus []string, apiIDs []string) map[string]any {
	return rolePayloadWithDataPerm(name, viewMenus, apiIDs, 1)
}

func rolePayloadWithDataPerm(name string, viewMenus []string, apiIDs []string, dataPerm int32) map[string]any {
	return map[string]any{
		"name":      name,
		"typ":       1,
		"dataPerm":  dataPerm,
		"desc":      "created by gateway e2e",
		"viewMenus": strings.Join(viewMenus, ","),
		"uses":      "",
		"apis":      apiIDs,
	}
}

func (e *environment) createUser(t *testing.T, token string, username string, phone string, deptID string, roleIDs []string) string {
	t.Helper()

	var out idResponse
	e.postJSON(t, "/api/user.v1.UserService/AddUser", map[string]any{
		"roleIds": roleIDs,
		"user": map[string]any{
			"username":   username,
			"name":       username,
			"phone":      phone,
			"gender":     1,
			"userStatus": 1,
			"deptId":     deptID,
			"remark":     "created by gateway e2e",
		},
	}, token, &out)
	if out.ID == "" || out.ID == "0" {
		t.Fatalf("AddUser returned invalid id %q", out.ID)
	}
	return out.ID
}

func (e *environment) loginAs(t *testing.T, username, password string) loginResponse {
	t.Helper()

	req := e.buildLoginRequest(t, username, password, "login")
	var out loginResponse
	e.postJSON(t, "/api/user.v1.UserService/Login", req, "", &out)
	if out.Token == "" {
		t.Fatalf("login as %s returned empty token", username)
	}
	return out
}

func defaultPasswordFromPhone(phone string) string {
	if len(phone) <= 6 {
		return phone
	}
	return phone[len(phone)-6:]
}

func (e *environment) waitForLog(t *testing.T, token string, username string, fragment string) logInfo {
	t.Helper()
	return e.waitForLogMatching(t, token, username, fragment, "")
}

func (e *environment) waitForLogMatching(t *testing.T, token string, username string, fragment string, paramsFragment string) logInfo {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	var last logListResponse
	for time.Now().Before(deadline) {
		e.postJSON(t, "/api/user.v1.LogService/GetLogList", map[string]any{
			"page":     1,
			"limit":    50,
			"sort":     "created_at",
			"order":    "desc",
			"username": username,
		}, token, &last)
		for _, item := range last.Logs {
			matchesAction := strings.Contains(item.Title, fragment) || strings.Contains(item.ModuleName, fragment) || strings.Contains(item.Typ, fragment)
			matchesParams := paramsFragment == "" || strings.Contains(item.Params, paramsFragment)
			if matchesAction && matchesParams {
				return item
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("log for username=%q fragment=%q paramsFragment=%q not found; last count=%d logs=%#v", username, fragment, paramsFragment, last.Count, last.Logs)
	return logInfo{}
}

func (e *environment) requireGETStatus(t *testing.T, path string, want int) []byte {
	t.Helper()

	resp, err := e.httpClient.Get(e.httpURL(path))
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("GET %s status = %d, want %d", path, resp.StatusCode, want)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET %s: %v", path, err)
	}
	return data
}
