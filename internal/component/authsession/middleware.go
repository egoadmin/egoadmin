package authsession

import (
	"context"
	"strings"

	"github.com/egoadmin/elib/pkg/constant"
	"github.com/egoadmin/elib/pkg/metadata"
	"github.com/egoadmin/elib/pkg/middleware"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
)

const bearerWord = "Bearer"

type MiddlewareOption func(*middlewareOptions)

type middlewareOptions struct {
	ignoreFn func(context.Context) bool
}

func WithIgnore(fn func(context.Context) bool) MiddlewareOption {
	return func(o *middlewareOptions) {
		o.ignoreFn = fn
	}
}

func (c *Component) Server(opts ...MiddlewareOption) middleware.Middleware {
	o := &middlewareOptions{}
	for _, opt := range opts {
		opt(o)
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			nextCtx, err := c.serverAuthContext(ctx, o)
			if err != nil {
				return nil, err
			}
			return handler(nextCtx, req)
		}
	}
}

func (c *Component) ServerStream(opts ...MiddlewareOption) middleware.StreamMiddleware {
	o := &middlewareOptions{}
	for _, opt := range opts {
		opt(o)
	}

	return func(sh middleware.StreamHandler) middleware.StreamHandler {
		return func(srv interface{}, stream grpc.ServerStream) error {
			wrappedStream := grpc_middleware.WrapServerStream(stream)
			nextCtx, err := c.serverAuthContext(stream.Context(), o)
			if err != nil {
				return err
			}
			wrappedStream.WrappedContext = nextCtx
			return sh(srv, wrappedStream)
		}
	}
}

func (c *Component) serverAuthContext(ctx context.Context, opts *middlewareOptions) (context.Context, error) {
	if opts != nil && opts.ignoreFn != nil && opts.ignoreFn(ctx) {
		return ctx, nil
	}

	rawToken, err := extractBearerToken(ctx)
	if err != nil {
		return nil, toEcode(ctx, err)
	}
	auth, err := c.ValidateAccessToken(ctx, rawToken)
	if err != nil {
		return nil, toEcode(ctx, err)
	}

	return NewContext(ctx, auth), nil
}

func extractBearerToken(ctx context.Context) (string, error) {
	value := metadata.ExtractIncoming(ctx).Get(constant.MDHeaderAuthorize)
	if value == "" {
		return "", ErrMissingToken
	}
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], bearerWord) || parts[1] == "" {
		return "", ErrMissingToken
	}
	return parts[1], nil
}
