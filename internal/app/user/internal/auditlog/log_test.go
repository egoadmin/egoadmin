package auditlog

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/authsession"
	elibmetadata "github.com/egoadmin/elib/pkg/metadata"
	elibgrpc "github.com/egoadmin/elib/pkg/transport/grpc"
)

func TestSaveSnapshotsContextValuesAfterParentCancel(t *testing.T) {
	t.Parallel()

	gotCh := make(chan AccessLogDetail, 1)
	ctxErrCh := make(chan error, 1)
	logger := New(func(ctx context.Context, alog AccessLogDetail) error {
		ctxErrCh <- ctx.Err()
		gotCh <- alog
		return nil
	})

	parent, cancel := context.WithCancel(context.Background())
	ctx := elibmetadata.New(map[string]string{
		urlHeader:           "/api/user/v1/User/Login",
		xForwardedForHeader: "127.0.0.1",
	}).ToIncoming(parent)
	ctx = elibgrpc.NewContext(ctx, "/user.v1.User/Login")
	ctx = authsession.NewContext(ctx, &authsession.AuthContext{
		UserID:   42,
		Username: "alice",
	})
	cancel()

	logger.Save(ctx, "用户-登录退出", "登录", "登录", loginAuditRequest{
		Username:       "alice",
		PasswordCipher: "secret",
	})

	select {
	case got := <-gotCh:
		if got.Username != "alice" {
			t.Fatalf("Username = %q, want alice", got.Username)
		}
		if got.UserID != "42" {
			t.Fatalf("UserID = %q, want 42", got.UserID)
		}
		if got.URL != "/api/user/v1/User/Login" {
			t.Fatalf("URL = %q, want /api/user/v1/User/Login", got.URL)
		}
		if got.OriginIP != "127.0.0.1" {
			t.Fatalf("OriginIP = %q, want 127.0.0.1", got.OriginIP)
		}
		if got.GrpcMethod != "/user.v1.User/Login" {
			t.Fatalf("GrpcMethod = %q, want /user.v1.User/Login", got.GrpcMethod)
		}
		if got.Action != "用户-登录退出.登录.登录" {
			t.Fatalf("Action = %q, want 用户-登录退出.登录.登录", got.Action)
		}

		var params map[string]string
		if err := json.Unmarshal([]byte(got.Params), &params); err != nil {
			t.Fatalf("Params unmarshal error: %v", err)
		}
		if params["username"] != "alice" {
			t.Fatalf("Params username = %q, want alice", params["username"])
		}
		if params["passwordCipher"] != "***" {
			t.Fatalf("Params passwordCipher = %q, want masked value", params["passwordCipher"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async audit log")
	}

	select {
	case err := <-ctxErrCh:
		if err != nil {
			t.Fatalf("writer context error = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async writer context")
	}
}

func TestSaveReturnsBeforeAsyncWriterCompletes(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	logger := New(func(ctx context.Context, alog AccessLogDetail) error {
		close(started)
		<-release
		return nil
	})

	returned := make(chan struct{})
	go func() {
		logger.Save(context.Background(), "模块", "操作", "动作", nil)
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Save blocked on the async writer")
	}

	close(release)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("async writer was not called")
	}
}

type loginAuditRequest struct {
	Username       string `json:"username"`
	PasswordCipher string `json:"passwordCipher"`
}
