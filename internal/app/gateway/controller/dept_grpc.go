package controller

import (
	"context"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
)

type DeptGRPC struct {
	client userclient.DeptService
}

func NewDeptGRPCController(client *userclient.Client) *DeptGRPC {
	return &DeptGRPC{client: client.Dept}
}

func (s *DeptGRPC) AddDept(ctx context.Context, in *userv1.AddDeptRequest) (*userv1.AddDeptResponse, error) {
	return s.client.AddDept(ctx, in)
}

func (s *DeptGRPC) DeleteDeptCascade(ctx context.Context, in *userv1.DeleteDeptCascadeRequest) (*userv1.DeleteDeptCascadeResponse, error) {
	return s.client.DeleteDeptCascade(ctx, in)
}

func (s *DeptGRPC) UpdateDept(ctx context.Context, in *userv1.UpdateDeptRequest) (*userv1.UpdateDeptResponse, error) {
	return s.client.UpdateDept(ctx, in)
}

func (s *DeptGRPC) UpdatePriorityDept(ctx context.Context, in *userv1.UpdatePriorityDeptRequest) (*userv1.UpdatePriorityDeptResponse, error) {
	return s.client.UpdatePriorityDept(ctx, in)
}

func (s *DeptGRPC) GetDeptByName(ctx context.Context, in *userv1.GetDeptByNameRequest) (*userv1.GetDeptByNameResponse, error) {
	return s.client.GetDeptByName(ctx, in)
}

func (s *DeptGRPC) GetDept(ctx context.Context, in *userv1.GetDeptRequest) (*userv1.GetDeptResponse, error) {
	return s.client.GetDept(ctx, in)
}

func (s *DeptGRPC) GetDeptTop(ctx context.Context, in *userv1.GetDeptTopRequest) (*userv1.GetDeptTopResponse, error) {
	return s.client.GetDeptTop(ctx, in)
}

func (s *DeptGRPC) GetDeptChild(ctx context.Context, in *userv1.GetDeptChildRequest) (*userv1.GetDeptChildResponse, error) {
	return s.client.GetDeptChild(ctx, in)
}

func (s *DeptGRPC) CheckDeptDelete(ctx context.Context, in *userv1.CheckDeptDeleteRequest) (*userv1.CheckDeptDeleteResponse, error) {
	return s.client.CheckDeptDelete(ctx, in)
}
