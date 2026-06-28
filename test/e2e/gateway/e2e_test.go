//go:build e2e

package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

var e2e *environment

func TestMain(m *testing.M) {
	if isCompileOnlyRun() {
		os.Exit(m.Run())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	var code int
	env, err := setupEnvironment(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup e2e environment: %v\n", err)
		code = 1
	} else {
		e2e = env
		code = m.Run()
	}

	if e2e != nil {
		if code != 0 {
			e2e.DumpDiagnostics()
		}
		if err := e2e.Cleanup(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup e2e environment: %v\n", err)
			if code == 0 {
				code = 1
			}
		}
	}

	os.Exit(code)
}

func isCompileOnlyRun() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-test.count=0" || arg == "-count=0" {
			return true
		}
	}
	return false
}

func TestSmokeGatewayReadiness(t *testing.T) {
	requireEnv(t)

	e2e.waitHTTPStatus(t, "/readyz", 200)
	e2e.waitHTTPStatus(t, "/healthz", 200)
	e2e.requireGETContains(t, "/openapi.yaml", "user.v1.UserService")
	e2e.requireGETContains(t, "/openapi.yaml", "/upload")
	e2e.requireGETContains(t, "/openapi.yaml", "/tus/upload")
	e2e.requireGETContains(t, "/openapi.yaml", "/cdn/file/{referenceId}")
	e2e.requireGETContains(t, "/openapi.yaml", "/cdn/image/{referenceId}")
	e2e.requireGETContains(t, "/", "<!doctype html")
}

func TestLoginAndSession(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	if admin.Token == "" {
		t.Fatal("login returned empty token")
	}
	if admin.RefreshToken == "" {
		t.Fatal("login returned empty refreshToken")
	}

	var center centerInfoResponse
	e2e.postJSON(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, admin.Token, &center)
	if center.User.Username != "admin" {
		t.Fatalf("center username = %q, want admin", center.User.Username)
	}

	resp := e2e.postRaw(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, "")
	requireErrorResponse(t, resp, "GetCenterInfo without token")

	resp = e2e.postRaw(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, "not-a-valid-token")
	requireErrorResponse(t, resp, "GetCenterInfo with invalid token")

	resp = e2e.postRaw(t, "/api/user.v1.UserService/Login", e2e.buildLoginRequest(t, "admin", "wrong-password", "login"), "")
	requireErrorResponse(t, resp, "Login with wrong password")

	var menus struct {
		Menus string `json:"menus"`
	}
	e2e.postJSON(t, "/api/user.v1.UserService/GetMenus", map[string]any{}, admin.Token, &menus)

	var refreshed loginResponse
	e2e.postJSON(t, "/api/user.v1.UserService/Login", map[string]any{
		"token": admin.RefreshToken,
		"ua":    e2eUA,
	}, "", &refreshed)
	if refreshed.Token == "" || refreshed.RefreshToken == "" {
		t.Fatalf("refresh login returned empty tokens: %#v", refreshed)
	}

	e2e.postJSON(t, "/api/user.v1.UserService/Logout", map[string]any{}, refreshed.Token, &emptyResponse{})

	resp = e2e.postRaw(t, "/api/user.v1.CenterService/GetCenterInfo", map[string]any{}, refreshed.Token)
	requireErrorResponse(t, resp, "GetCenterInfo after logout")
}

func TestLoginCryptoReplayRejected(t *testing.T) {
	requireEnv(t)

	req := e2e.buildLoginRequest(t, "admin", "123456", "login")
	var first loginResponse
	e2e.postJSON(t, "/api/user.v1.UserService/Login", req, "", &first)
	if first.Token == "" {
		t.Fatal("first login returned empty token")
	}

	resp := e2e.postRaw(t, "/api/user.v1.UserService/Login", req, "")
	requireErrorResponse(t, resp, "replayed login crypto challenge")
}

func TestAPICatalog(t *testing.T) {
	requireEnv(t)

	admin := e2e.loginAsAdmin(t)
	var got apisResponse
	e2e.postJSON(t, "/api/user.v1.UserService/GetAPIs", map[string]any{}, admin.Token, &got)
	if len(got.Apis) == 0 {
		t.Fatal("GetAPIs returned empty api list")
	}

	want := "USER.V1.USERSERVICE/LOGIN"
	for _, api := range got.Apis {
		if api.FullPath == want {
			return
		}
	}
	t.Fatalf("GetAPIs did not contain %s; got %d apis", want, len(got.Apis))
}

func requireEnv(t *testing.T) {
	t.Helper()
	if e2e == nil {
		t.Fatal("e2e environment is not initialized")
	}
}
