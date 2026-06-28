package i18n

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestMessageUsesAcceptLanguage(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderAcceptLanguage, "en-US,en;q=0.9"))

	got := Message(ctx, "LoginExpired")
	if got != "Login has expired" {
		t.Fatalf("Message() = %q, want Login has expired", got)
	}
}

func TestMessageUsesGatewayAcceptLanguage(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(headerGatewayAcceptLanguage, "en-US,en;q=0.9"))

	got := Message(ctx, "LoginInvalid")
	if got != "Login is invalid" {
		t.Fatalf("Message() = %q, want Login is invalid", got)
	}
}

func TestWithAcceptLanguageAddsIncomingMetadata(t *testing.T) {
	ctx := WithAcceptLanguage(context.Background(), "en-US,en;q=0.9")

	got := Message(ctx, "FileTooLarge")
	if got != "File upload failed: file is too large" {
		t.Fatalf("Message() = %q, want File upload failed: file is too large", got)
	}
}

func TestMessageDefaultsToChinese(t *testing.T) {
	got := Message(context.Background(), "LoginExpired")
	if got != "登录已过期" {
		t.Fatalf("Message() = %q, want 登录已过期", got)
	}
}
