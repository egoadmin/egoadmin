package controller

import (
	"context"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
)

type LogGRPC struct {
	client userclient.LogService
}

func NewLogGRPCController(client *userclient.Client) *LogGRPC {
	return &LogGRPC{client: client.Log}
}

func (s *LogGRPC) GetLogList(ctx context.Context, in *userv1.GetLogListRequest) (*userv1.GetLogListResponse, error) {
	return s.client.GetLogList(ctx, in)
}
