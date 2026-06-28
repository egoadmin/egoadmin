package userclient

import (
	"context"
	"errors"
	"testing"

	ecode "github.com/egoadmin/elib/api/gen/go/ecode/v1"
	"github.com/gotomicro/ego/core/eerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestOutgoingContextCopiesIncomingMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer access-token",
		"accept-language", "en-US,en;q=0.9",
		"x-forwarded-for", "127.0.0.1",
		"traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00",
	))

	out := outgoingContext(ctx)
	md, ok := metadata.FromOutgoingContext(out)
	if !ok {
		t.Fatal("outgoing metadata missing")
	}
	if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer access-token" {
		t.Fatalf("authorization metadata = %#v, want Bearer access-token", got)
	}
	if got := md.Get("accept-language"); len(got) != 1 || got[0] != "en-US,en;q=0.9" {
		t.Fatalf("accept-language metadata = %#v, want en-US,en;q=0.9", got)
	}
	if got := md.Get("x-forwarded-for"); len(got) != 1 || got[0] != "127.0.0.1" {
		t.Fatalf("x-forwarded-for metadata = %#v, want 127.0.0.1", got)
	}
	if got := md.Get("traceparent"); len(got) != 1 || got[0] != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00" {
		t.Fatalf("traceparent metadata = %#v, want traceparent", got)
	}
}

func TestOutgoingContextDropsHTTPOnlyMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer access-token",
		"connection", "keep-alive",
		"content-type", "application/json",
		"grpcgateway-user-agent", "node",
		"accept-encoding", "gzip",
	))

	out := outgoingContext(ctx)
	md, ok := metadata.FromOutgoingContext(out)
	if !ok {
		t.Fatal("outgoing metadata missing")
	}
	if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer access-token" {
		t.Fatalf("authorization metadata = %#v, want Bearer access-token", got)
	}
	for _, key := range []string{"connection", "content-type", "grpcgateway-user-agent", "accept-encoding"} {
		if got := md.Get(key); len(got) != 0 {
			t.Fatalf("%s metadata = %#v, want dropped", key, got)
		}
	}
}

func TestOutgoingContextPreservesExistingOutgoingMetadata(t *testing.T) {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("app", "egoadmin-gateway"))
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer access-token"))

	out := outgoingContext(ctx)
	md, ok := metadata.FromOutgoingContext(out)
	if !ok {
		t.Fatal("outgoing metadata missing")
	}
	if got := md.Get("app"); len(got) != 1 || got[0] != "egoadmin-gateway" {
		t.Fatalf("app metadata = %#v, want egoadmin-gateway", got)
	}
	if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer access-token" {
		t.Fatalf("authorization metadata = %#v, want Bearer access-token", got)
	}
}

func TestNormalizeEgoErrorPreservesRemoteEcode(t *testing.T) {
	const wantMessage = "Cannot assign data permissions outside your own scope"

	remoteErr := ecode.ErrorAccessDenied().WithMessage(wantMessage)
	err := normalizeEgoError(status.Convert(remoteErr).Err())
	egoErr := eerrors.FromError(err)

	wantReason := eerrors.FromError(ecode.ErrorAccessDenied()).GetReason()
	if egoErr.GetReason() != wantReason {
		t.Fatalf("reason = %q, want %q", egoErr.GetReason(), wantReason)
	}
	if egoErr.GetMessage() != wantMessage {
		t.Fatalf("message = %q, want %q", egoErr.GetMessage(), wantMessage)
	}
}

func TestNormalizeEgoErrorKeepsPlainErrors(t *testing.T) {
	plainErr := errors.New("plain error")
	if got := normalizeEgoError(plainErr); got != plainErr {
		t.Fatalf("plain error normalized to %T, want original", got)
	}

	statusErr := status.Error(codes.Unavailable, "user service unavailable")
	if got := normalizeEgoError(statusErr); got != statusErr {
		t.Fatalf("status error normalized to %T, want original", got)
	}
}
