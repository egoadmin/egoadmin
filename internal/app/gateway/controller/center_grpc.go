package controller

import (
	"context"
	"errors"
	"fmt"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	userclient "github.com/egoadmin/egoadmin/internal/client/userclient"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/component/idgen/idcodec"
	uploadcomponent "github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/gotomicro/ego/core/elog"
)

type CenterGRPC struct {
	client userclient.CenterService
	upload *uploadcomponent.Component
	codec  idcodec.Interface
}

func NewCenterGRPCController(client *userclient.Client, upload *uploadcomponent.Component, codec *idcodec.Component) *CenterGRPC {
	return &CenterGRPC{client: client.Center, upload: upload, codec: codec}
}

func (s *CenterGRPC) GetCenterInfo(ctx context.Context, in *userv1.GetCenterInfoRequest) (*userv1.GetCenterInfoResponse, error) {
	out, err := s.client.GetCenterInfo(ctx, in)
	if err != nil {
		return nil, err
	}
	if out.GetUser() == nil {
		return out, nil
	}
	auth, ok := authsession.FromContext(ctx)
	if !ok {
		return out, nil
	}
	reference, err := s.upload.GetBoundReference(ctx, uploadcomponent.ReferenceBinding{
		Profile:      "avatar",
		Service:      "user",
		ResourceType: "user",
		ResourceID:   auth.UserID,
		FieldName:    "avatar",
	})
	if err != nil {
		if !errors.Is(err, uploadcomponent.ErrReferenceNotFound) {
			elog.Error("get avatar upload reference failed", elog.FieldErr(err))
		}
		if out.User.GetAvatar() != "" {
			out.User.Avatar = ""
			out.User.AvatarReferenceId = ""
		}
		return out, nil
	}
	if publicReferenceID := s.upload.PublicReferenceID(reference.ReferenceID); publicReferenceID != "" {
		out.User.AvatarReferenceId = publicReferenceID
		out.User.Avatar = "/cdn/image/" + publicReferenceID
	}
	return out, nil
}

func (s *CenterGRPC) EditCenterInfo(ctx context.Context, in *userv1.EditCenterInfoRequest) (*userv1.EditCenterInfoResponse, error) {
	return s.client.EditCenterInfo(ctx, in)
}

func (s *CenterGRPC) EditCenterPassword(ctx context.Context, in *userv1.EditCenterPasswordRequest) (*userv1.EditCenterPasswordResponse, error) {
	return s.client.EditCenterPassword(ctx, in)
}

func (s *CenterGRPC) EditCenterAvatar(ctx context.Context, in *userv1.EditCenterAvatarRequest) (*userv1.EditCenterAvatarResponse, error) {
	if in.GetReferenceId() == "" {
		return nil, fmt.Errorf("upload: reference id is required")
	}
	auth, ok := authsession.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("upload: auth context is required")
	}
	referenceID, err := uploadcomponent.DecodeReferenceID(s.codec, in.GetReferenceId())
	if err != nil {
		return nil, err
	}
	reference, err := s.upload.GetReference(ctx, referenceID, auth.UserID)
	if err != nil {
		return nil, err
	}
	if reference.Profile != "avatar" {
		return nil, fmt.Errorf("upload: reference profile mismatch")
	}
	committed, err := s.upload.CommitReference(ctx, uploadcomponent.ReferenceBinding{
		ReferenceID:          referenceID,
		OwnerUserID:          auth.UserID,
		Profile:              "avatar",
		Service:              "user",
		ResourceType:         "user",
		ResourceID:           auth.UserID,
		FieldName:            "avatar",
		DeferReleaseExisting: true,
	})
	if err != nil {
		return nil, err
	}
	publicReferenceID := s.upload.PublicReferenceID(committed.ReferenceID)
	if publicReferenceID == "" {
		_ = s.upload.ReleaseReference(context.WithoutCancel(ctx), referenceID, auth.UserID)
		return nil, fmt.Errorf("upload: encode reference id failed")
	}
	out, err := s.client.EditCenterAvatar(ctx, &userv1.EditCenterAvatarRequest{ReferenceId: publicReferenceID})
	if err != nil {
		_ = s.upload.ReleaseReference(context.WithoutCancel(ctx), referenceID, auth.UserID)
		return nil, err
	}
	if err = s.upload.ReleasePreviousReferences(context.WithoutCancel(ctx), uploadcomponent.ReferenceBinding{
		ReferenceID:  referenceID,
		Service:      "user",
		ResourceType: "user",
		ResourceID:   auth.UserID,
		FieldName:    "avatar",
	}); err != nil {
		elog.Error("release previous avatar upload references failed", elog.FieldErr(err))
	}
	return out, nil
}
