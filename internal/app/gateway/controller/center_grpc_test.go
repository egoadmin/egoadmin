package controller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	userv1 "github.com/egoadmin/egoadmin/api/gen/go/user/v1"
	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/component/idgen/idcodec"
	uploadcomponent "github.com/egoadmin/egoadmin/internal/component/upload"
)

func TestCenterGRPC_EditCenterAvatarUsesUploadReference(t *testing.T) {
	upload, store := newFakeUploadComponent(t)
	client := &fakeCenterService{}
	codec := newTestIDCodec(t)
	ctrl := &CenterGRPC{client: client, upload: upload, codec: codec}
	ctx := authsession.NewContext(context.Background(), &authsession.AuthContext{UserID: 20001})
	referenceID := mustEncodeReferenceID(t, codec, 99)

	_, err := ctrl.EditCenterAvatar(ctx, &userv1.EditCenterAvatarRequest{ReferenceId: referenceID})
	if err != nil {
		t.Fatalf("EditCenterAvatar() error = %v", err)
	}
	if client.avatar == nil || client.avatar.GetReferenceId() != referenceID {
		t.Fatalf("forwarded request = %#v, want public reference id %q", client.avatar, referenceID)
	}
	if store.committed == nil || store.committed.ReferenceID != 99 {
		t.Fatalf("committed = %#v, want reference 99", store.committed)
	}
	if !store.committed.DeferReleaseExisting {
		t.Fatalf("committed.DeferReleaseExisting = false, want true for cross-service save")
	}
	if store.previousReleased == nil || store.previousReleased.ReferenceID != 99 {
		t.Fatalf("previousReleased = %#v, want old binding release after save", store.previousReleased)
	}
}

func TestCenterGRPC_GetCenterInfoAddsAvatarReferenceID(t *testing.T) {
	upload, store := newFakeUploadComponent(t)
	store.boundReference = &uploadcomponent.ReferenceDetail{
		ReferenceID: 99,
		Profile:     "avatar",
		Status:      uploadcomponent.ReferenceStatusBound,
		ObjectKey:   "files/avatar/2026/06/26/file-test-99.png",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	client := &fakeCenterService{
		centerInfo: &userv1.GetCenterInfoResponse{
			User: &userv1.User{Avatar: "files/avatar/2026/06/26/file-test-99.png"},
		},
	}
	ctrl := &CenterGRPC{client: client, upload: upload, codec: newTestIDCodec(t)}
	ctx := authsession.NewContext(context.Background(), &authsession.AuthContext{UserID: 20001})

	out, err := ctrl.GetCenterInfo(ctx, &userv1.GetCenterInfoRequest{})
	if err != nil {
		t.Fatalf("GetCenterInfo() error = %v", err)
	}
	wantReferenceID := upload.PublicReferenceID(99)
	if out.GetUser().GetAvatarReferenceId() != wantReferenceID {
		t.Fatalf("avatarReferenceId = %q, want %q", out.GetUser().GetAvatarReferenceId(), wantReferenceID)
	}
	if got, want := out.GetUser().GetAvatar(), "/cdn/image/"+wantReferenceID; got != want {
		t.Fatalf("avatar = %q, want %q", got, want)
	}
}

func TestCenterGRPC_GetCenterInfoClearsStoredAvatarWithoutBoundReference(t *testing.T) {
	upload, store := newFakeUploadComponent(t)
	store.boundReferenceErr = uploadcomponent.ErrReferenceNotFound
	client := &fakeCenterService{
		centerInfo: &userv1.GetCenterInfoResponse{
			User: &userv1.User{Avatar: "legacy/path.png"},
		},
	}
	ctrl := &CenterGRPC{client: client, upload: upload, codec: newTestIDCodec(t)}
	ctx := authsession.NewContext(context.Background(), &authsession.AuthContext{UserID: 20001})

	out, err := ctrl.GetCenterInfo(ctx, &userv1.GetCenterInfoRequest{})
	if err != nil {
		t.Fatalf("GetCenterInfo() error = %v", err)
	}
	if out.GetUser().GetAvatarReferenceId() != "" || out.GetUser().GetAvatar() != "" {
		t.Fatalf("user = %#v, want avatar fields hidden without bound upload reference", out.GetUser())
	}
}

func TestCenterGRPC_EditCenterAvatarReleasesReferenceWhenUserSaveFails(t *testing.T) {
	upload, store := newFakeUploadComponent(t)
	client := &fakeCenterService{avatarErr: io.ErrUnexpectedEOF}
	codec := newTestIDCodec(t)
	ctrl := &CenterGRPC{client: client, upload: upload, codec: codec}
	ctx := authsession.NewContext(context.Background(), &authsession.AuthContext{UserID: 20001})
	referenceID := mustEncodeReferenceID(t, codec, 99)

	_, err := ctrl.EditCenterAvatar(ctx, &userv1.EditCenterAvatarRequest{ReferenceId: referenceID})
	if err == nil {
		t.Fatalf("EditCenterAvatar() error = nil, want user save error")
	}
	if store.released == nil || store.released.ReferenceID != 99 {
		t.Fatalf("released = %#v, want reference 99", store.released)
	}
	if store.previousReleased != nil {
		t.Fatalf("previousReleased = %#v, want nil when user save fails", store.previousReleased)
	}
}

func TestCenterGRPC_EditCenterAvatarKeepsSuccessWhenPreviousReleaseFails(t *testing.T) {
	upload, store := newFakeUploadComponent(t)
	store.previousReleaseErr = errors.New("release previous failed")
	client := &fakeCenterService{}
	codec := newTestIDCodec(t)
	ctrl := &CenterGRPC{client: client, upload: upload, codec: codec}
	ctx := authsession.NewContext(context.Background(), &authsession.AuthContext{UserID: 20001})
	referenceID := mustEncodeReferenceID(t, codec, 99)

	_, err := ctrl.EditCenterAvatar(ctx, &userv1.EditCenterAvatarRequest{ReferenceId: referenceID})
	if err != nil {
		t.Fatalf("EditCenterAvatar() error = %v, want nil because user save already succeeded", err)
	}
	if store.previousReleased == nil || store.previousReleased.ReferenceID != 99 {
		t.Fatalf("previousReleased = %#v, want release attempted", store.previousReleased)
	}
}

func TestCenterGRPC_EditCenterAvatarRejectsMissingReference(t *testing.T) {
	client := &fakeCenterService{}
	upload, _ := newFakeUploadComponent(t)
	ctrl := &CenterGRPC{client: client, upload: upload, codec: newTestIDCodec(t)}

	_, err := ctrl.EditCenterAvatar(context.Background(), &userv1.EditCenterAvatarRequest{})
	if err == nil {
		t.Fatalf("EditCenterAvatar() error = nil, want missing reference error")
	}
	if client.avatar != nil {
		t.Fatalf("forwarded request = %#v, want nil", client.avatar)
	}
}

func newFakeUploadComponent(t *testing.T) (*uploadcomponent.Component, *fakeUploadMetadataStore) {
	t.Helper()
	store := &fakeUploadMetadataStore{}
	component, err := uploadcomponent.New(store, &fakeUploadObjectStore{}, fakeUploadFlake(1), uploadcomponent.WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("upload.New() error = %v", err)
	}
	return component, store
}

func newTestIDCodec(t *testing.T) *idcodec.Component {
	t.Helper()
	cfg := idcodec.DefaultConfig()
	cfg.Secret = "0123456789abcdef0123456789abcdef"
	cfg.EnableMetrics = false
	return idcodec.DefaultContainer().Build(idcodec.WithConfig(cfg), idcodec.WithName("component.idgen.codec.gateway.controller.test"))
}

func mustEncodeReferenceID(t *testing.T, codec idcodec.Interface, id int64) string {
	t.Helper()
	out, err := codec.Encode(uploadcomponent.PublicReferenceIDPrefix(), id)
	if err != nil {
		t.Fatalf("encode reference id: %v", err)
	}
	return out
}

type fakeCenterService struct {
	centerInfo *userv1.GetCenterInfoResponse
	avatar     *userv1.EditCenterAvatarRequest
	avatarErr  error
}

func (s *fakeCenterService) GetCenterInfo(context.Context, *userv1.GetCenterInfoRequest) (*userv1.GetCenterInfoResponse, error) {
	if s.centerInfo != nil {
		return s.centerInfo, nil
	}
	return &userv1.GetCenterInfoResponse{}, nil
}

func (s *fakeCenterService) EditCenterInfo(context.Context, *userv1.EditCenterInfoRequest) (*userv1.EditCenterInfoResponse, error) {
	return &userv1.EditCenterInfoResponse{}, nil
}

func (s *fakeCenterService) EditCenterPassword(context.Context, *userv1.EditCenterPasswordRequest) (*userv1.EditCenterPasswordResponse, error) {
	return &userv1.EditCenterPasswordResponse{}, nil
}

func (s *fakeCenterService) EditCenterAvatar(_ context.Context, in *userv1.EditCenterAvatarRequest) (*userv1.EditCenterAvatarResponse, error) {
	cp := *in
	s.avatar = &cp
	if s.avatarErr != nil {
		return nil, s.avatarErr
	}
	return &userv1.EditCenterAvatarResponse{}, nil
}

type fakeUploadMetadataStore struct {
	committed          *uploadcomponent.CommitReferenceCommand
	released           *uploadcomponent.ReleaseReferenceCommand
	previousReleased   *uploadcomponent.ReleasePreviousReferencesCommand
	previousReleaseErr error
	boundReference     *uploadcomponent.ReferenceDetail
	boundReferenceErr  error
}

func (s *fakeUploadMetadataStore) CreateMultipart(context.Context, uploadcomponent.CreateMultipartCommand) (*uploadcomponent.CreateMultipartResult, error) {
	return nil, nil
}

func (s *fakeUploadMetadataStore) CreateTus(context.Context, uploadcomponent.CreateTusCommand) (*uploadcomponent.CreateTusResult, error) {
	return nil, nil
}

func (s *fakeUploadMetadataStore) CreateInstantReference(context.Context, uploadcomponent.CreateInstantReferenceCommand) (*uploadcomponent.CreateInstantReferenceResult, error) {
	return nil, nil
}

func (s *fakeUploadMetadataStore) CommitReference(_ context.Context, cmd uploadcomponent.CommitReferenceCommand) (*uploadcomponent.ReferenceDetail, error) {
	cp := cmd
	s.committed = &cp
	return &uploadcomponent.ReferenceDetail{
		ReferenceID: cmd.ReferenceID,
		OwnerUserID: cmd.OwnerUserID,
		Profile:     cmd.Profile,
		Status:      uploadcomponent.ReferenceStatusBound,
		ObjectKey:   "files/avatar/2026/06/26/file-test-99.png",
		ExpiresAt:   time.Now().Add(time.Hour),
	}, nil
}

func (s *fakeUploadMetadataStore) ReleasePreviousReferences(_ context.Context, cmd uploadcomponent.ReleasePreviousReferencesCommand) error {
	cp := cmd
	s.previousReleased = &cp
	return s.previousReleaseErr
}

func (s *fakeUploadMetadataStore) GetReference(_ context.Context, referenceID uint64, ownerUserID uint64) (*uploadcomponent.ReferenceDetail, error) {
	return &uploadcomponent.ReferenceDetail{
		ReferenceID: referenceID,
		OwnerUserID: ownerUserID,
		Profile:     "avatar",
		Status:      uploadcomponent.ReferenceStatusTemporary,
		ObjectKey:   "files/avatar/2026/06/26/file-test-99.png",
		ExpiresAt:   time.Now().Add(time.Hour),
	}, nil
}

func (s *fakeUploadMetadataStore) GetBoundReference(context.Context, uploadcomponent.GetBoundReferenceCommand) (*uploadcomponent.ReferenceDetail, error) {
	if s.boundReferenceErr != nil {
		return nil, s.boundReferenceErr
	}
	if s.boundReference != nil {
		return s.boundReference, nil
	}
	return nil, uploadcomponent.ErrReferenceNotFound
}

func (s *fakeUploadMetadataStore) GetDownloadReference(context.Context, uint64) (*uploadcomponent.DownloadObject, error) {
	return &uploadcomponent.DownloadObject{}, nil
}

func (s *fakeUploadMetadataStore) GetDownloadReferenceForOwner(context.Context, uint64, uint64) (*uploadcomponent.DownloadObject, error) {
	return &uploadcomponent.DownloadObject{}, nil
}

func (s *fakeUploadMetadataStore) ReleaseReference(_ context.Context, cmd uploadcomponent.ReleaseReferenceCommand) error {
	cp := cmd
	s.released = &cp
	return nil
}

func (s *fakeUploadMetadataStore) ExpireTemporaryReferences(context.Context, time.Time, int) ([]uploadcomponent.ExpiredReference, error) {
	return nil, nil
}

func (s *fakeUploadMetadataStore) FindUnreferencedObjects(context.Context, int) ([]uploadcomponent.UnreferencedObject, error) {
	return nil, nil
}

func (s *fakeUploadMetadataStore) MarkObjectDeleting(context.Context, uint64) (bool, error) {
	return false, nil
}

func (s *fakeUploadMetadataStore) MarkObjectDeleted(context.Context, uint64) error {
	return nil
}

func (s *fakeUploadMetadataStore) FindReusableObject(context.Context, uploadcomponent.FindReusableObjectCommand) (*uploadcomponent.ReusableObject, error) {
	return nil, nil
}

func (s *fakeUploadMetadataStore) MarkObjectAvailable(context.Context, uint64) error {
	return nil
}

func (s *fakeUploadMetadataStore) MarkSessionFinished(context.Context, uint64) error {
	return nil
}

func (s *fakeUploadMetadataStore) MarkUploadFailed(context.Context, uint64) error {
	return nil
}

func (s *fakeUploadMetadataStore) MarkTusUploadFinished(context.Context, uploadcomponent.MarkTusUploadFinishedCommand) (*uploadcomponent.TusUploadDetail, error) {
	return nil, nil
}

func (s *fakeUploadMetadataStore) MarkTusUploadCreated(context.Context, uploadcomponent.MarkTusUploadCreatedCommand) error {
	return nil
}

func (s *fakeUploadMetadataStore) FindExpiredTusUploads(context.Context, time.Time, int) ([]uploadcomponent.ExpiredTusUpload, error) {
	return nil, nil
}

func (s *fakeUploadMetadataStore) MarkTusUploadAborted(context.Context, uint64) error {
	return nil
}

func (s *fakeUploadMetadataStore) FindTusMetadataForCleanup(context.Context, int) ([]uploadcomponent.TusMetadataObject, error) {
	return nil, nil
}

func (s *fakeUploadMetadataStore) MarkTusMetadataCleaning(context.Context, uint64) (bool, error) {
	return false, nil
}

func (s *fakeUploadMetadataStore) MarkTusMetadataCleaned(context.Context, uint64) error {
	return nil
}

type fakeUploadObjectStore struct{}

type fakeUploadObjectReader struct {
	*bytes.Reader
}

func (s *fakeUploadObjectStore) Put(context.Context, string, io.Reader, int64, uploadcomponent.PutOptions) error {
	return nil
}

func (s *fakeUploadObjectStore) Get(context.Context, string) (uploadcomponent.ObjectReader, error) {
	return &fakeUploadObjectReader{Reader: bytes.NewReader(nil)}, nil
}

func (s *fakeUploadObjectStore) Delete(context.Context, string) error {
	return nil
}

func (s *fakeUploadObjectStore) Stat(context.Context, string) (uploadcomponent.ObjectInfo, error) {
	return uploadcomponent.ObjectInfo{}, nil
}

func (r *fakeUploadObjectReader) Close() error {
	return nil
}

func (r *fakeUploadObjectReader) Stat() (uploadcomponent.ObjectInfo, error) {
	return uploadcomponent.ObjectInfo{}, nil
}

type fakeUploadFlake uint64

func (f fakeUploadFlake) Get() (uint64, error) {
	return uint64(f), nil
}
