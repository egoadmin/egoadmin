package i18n

import (
	"context"
	"embed"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"google.golang.org/grpc/metadata"

	ecode "github.com/egoadmin/elib/api/gen/go/ecode/v1"
)

const (
	HeaderAcceptLanguage        = "accept-language"
	headerGatewayAcceptLanguage = "grpcgateway-accept-language"
	defaultLanguage             = "zh-CN"
)

//go:embed locales/*.toml
var localeFS embed.FS

var bundle = newBundle()

func newBundle() *i18n.Bundle {
	b := i18n.NewBundle(language.SimplifiedChinese)
	b.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	mustLoadMessageFileFS(b, "locales/active.zh-CN.toml")
	mustLoadMessageFileFS(b, "locales/active.en.toml")
	return b
}

func mustLoadMessageFileFS(b *i18n.Bundle, path string) {
	if _, err := b.LoadMessageFileFS(localeFS, path); err != nil {
		panic(err)
	}
}

func Message(ctx context.Context, messageID string) string {
	return Localize(ctx, messageID, nil)
}

func Localize(ctx context.Context, messageID string, data map[string]any) string {
	msg, err := localizer(ctx).Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: data,
	})
	if err != nil {
		return messageID
	}
	return msg
}

func ErrorFailed(ctx context.Context, messageID string, data map[string]any) error {
	return ecode.ErrorFailed().WithMessage(Localize(ctx, messageID, data))
}

func ErrorNotLogin(ctx context.Context, messageID string, data map[string]any) error {
	return ecode.ErrorNotLogin().WithMessage(Localize(ctx, messageID, data))
}

func ErrorAccessDenied(ctx context.Context, messageID string, data map[string]any) error {
	return ecode.ErrorAccessDenied().WithMessage(Localize(ctx, messageID, data))
}

func WithAcceptLanguage(ctx context.Context, value string) context.Context {
	value = strings.TrimSpace(value)
	if value == "" {
		return ctx
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}
	md.Set(HeaderAcceptLanguage, value)
	return metadata.NewIncomingContext(ctx, md)
}

func localizer(ctx context.Context) *i18n.Localizer {
	return i18n.NewLocalizer(bundle, languages(ctx)...)
}

func languages(ctx context.Context) []string {
	values := make([]string, 0, 2)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if lang := firstMetadataValue(md, HeaderAcceptLanguage, headerGatewayAcceptLanguage); lang != "" {
			values = append(values, lang)
		}
	}
	values = append(values, defaultLanguage)
	return values
}

func firstMetadataValue(md metadata.MD, keys ...string) string {
	for _, key := range keys {
		for _, value := range md.Get(key) {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}
