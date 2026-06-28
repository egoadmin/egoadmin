package upload

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/server/egin"
	tusd "github.com/tus/tusd/v2/pkg/handler"
)

func TestRegisterTusRoutesCreatesUploadMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeMetadataStore{}
	component, err := New(store, &fakeObjectStore{}, fakeFlake(1001),
		WithBucket("egoadmin"),
		WithTusS3API(&fakeTusS3API{}),
		WithIDCodec(newTestIDCodec(t)),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cfg := component.Config()
	cfg.Tus.Enabled = true
	cfg.Tus.Path = "/tus/upload"
	cfg.Tus.ObjectPrefix = "files"
	cfg.Tus.MetadataPrefix = "tus-meta"

	engine := gin.New()
	cc := &egin.Component{Engine: engine}
	err = RegisterTusRoutes(cc, component, MultipartOptions{
		BeforeTusHandle: func(context.Context, http.Header) (*AuthContext, error) {
			return &AuthContext{UserID: 20001}, nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterTusRoutes() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/tus/upload", nil)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", "12")
	req.Header.Set("Upload-Metadata", "filename YXZhdGFyLnBuZw==,profile YXZhdGFy,filetype aW1hZ2UvcG5n")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /tus/upload status = %d body = %q", rec.Code, rec.Body.String())
	}
	wantReferenceID := component.PublicReferenceID(2)
	if rec.Header().Get("X-Upload-Reference-Id") != wantReferenceID {
		t.Fatalf("X-Upload-Reference-Id = %q, want %q", rec.Header().Get("X-Upload-Reference-Id"), wantReferenceID)
	}
	if got := rec.Header().Get("X-Upload-Object-Key"); got != "" {
		t.Fatalf("X-Upload-Object-Key = %q, want empty public response", got)
	}
	if rec.Header().Get("X-Upload-Status") != ReferenceStatusTemporary || rec.Header().Get("X-Upload-Expires-At") == "" {
		t.Fatalf("upload status/expires headers = %q/%q", rec.Header().Get("X-Upload-Status"), rec.Header().Get("X-Upload-Expires-At"))
	}
	if store.createdTus == nil || store.createdTus.OwnerUserID != 20001 || store.createdTus.Profile != "avatar" {
		t.Fatalf("createdTus = %#v, want owner/profile metadata", store.createdTus)
	}
	if !strings.HasPrefix(store.createdTus.ObjectKey, "files/avatar/") || !strings.HasPrefix(store.createdTus.TusInfoKey, "tus-meta/avatar/") {
		t.Fatalf("createdTus keys = object %q info %q", store.createdTus.ObjectKey, store.createdTus.TusInfoKey)
	}
}

func TestTusRoutesRequireAuthForPatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	component, err := New(&fakeMetadataStore{}, &fakeObjectStore{}, fakeFlake(1001),
		WithBucket("egoadmin"),
		WithTusS3API(&fakeTusS3API{}),
		WithIDCodec(newTestIDCodec(t)),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cfg := component.Config()
	cfg.Tus.Enabled = true
	cfg.Tus.Path = "/tus/upload"
	cfg.Tus.ObjectPrefix = "files"
	cfg.Tus.MetadataPrefix = "tus-meta"

	engine := gin.New()
	cc := &egin.Component{Engine: engine}
	err = RegisterTusRoutes(cc, component, MultipartOptions{
		BeforeTusHandle: func(context.Context, http.Header) (*AuthContext, error) {
			return nil, errors.New("missing auth")
		},
	})
	if err != nil {
		t.Fatalf("RegisterTusRoutes() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/tus/upload/avatar/2026/06/26/file-test-1001.png+multipart", strings.NewReader("part"))
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Offset", "0")
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("PATCH /tus/upload status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestTusServerCompleteUploadMarksMetadataFinished(t *testing.T) {
	store := &fakeMetadataStore{}
	component, err := New(store, &fakeObjectStore{}, fakeFlake(1001),
		WithBucket("egoadmin"),
		WithTusS3API(&fakeTusS3API{}),
		WithIDCodec(newTestIDCodec(t)),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cfg := component.Config()
	cfg.Tus.Enabled = true
	cfg.Tus.Path = "/tus/upload"
	cfg.Tus.ObjectPrefix = "files"
	cfg.Tus.MetadataPrefix = "tus-meta"

	server, err := newTusServer(component, MultipartOptions{})
	if err != nil {
		t.Fatalf("newTusServer() error = %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	server.handler.CompleteUploads <- tusHookEvent("avatar/2026/06/26/file-test-1001.png+multipart", "files/avatar/2026/06/26/file-test-1001.png")
	deadline := time.After(time.Second)
	var finishedTus *MarkTusUploadFinishedCommand
	for finishedTus == nil {
		select {
		case <-deadline:
			t.Fatalf("finishedTus was not recorded")
		default:
			finishedTus = store.currentFinishedTus()
			time.Sleep(10 * time.Millisecond)
		}
	}
	if finishedTus.TusUploadID != "avatar/2026/06/26/file-test-1001.png+multipart" || finishedTus.ObjectKey != "files/avatar/2026/06/26/file-test-1001.png" {
		t.Fatalf("finishedTus = %#v", finishedTus)
	}
}

func TestTusServerCreatedUploadStoresCompleteUploadID(t *testing.T) {
	store := &fakeMetadataStore{}
	component, err := New(store, &fakeObjectStore{}, fakeFlake(1001),
		WithBucket("egoadmin"),
		WithTusS3API(&fakeTusS3API{}),
		WithIDCodec(newTestIDCodec(t)),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cfg := component.Config()
	cfg.Tus.Enabled = true
	cfg.Tus.Path = "/tus/upload"
	cfg.Tus.ObjectPrefix = "files"
	cfg.Tus.MetadataPrefix = "tus-meta"

	server, err := newTusServer(component, MultipartOptions{})
	if err != nil {
		t.Fatalf("newTusServer() error = %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	server.handler.CreatedUploads <- tusHookEvent("avatar/2026/06/26/file-test-1001.png+multipart", "files/avatar/2026/06/26/file-test-1001.png")
	deadline := time.After(time.Second)
	var tusCreated *MarkTusUploadCreatedCommand
	for tusCreated == nil {
		select {
		case <-deadline:
			t.Fatalf("tusCreated was not recorded")
		default:
			tusCreated = store.currentTusCreated()
			time.Sleep(10 * time.Millisecond)
		}
	}
	if tusCreated.TusUploadID != "avatar/2026/06/26/file-test-1001.png+multipart" || tusCreated.ObjectKey != "files/avatar/2026/06/26/file-test-1001.png" {
		t.Fatalf("tusCreated = %#v", tusCreated)
	}
}

func TestTusServerMaxSizeUsesAllProfiles(t *testing.T) {
	component, err := New(&fakeMetadataStore{}, &fakeObjectStore{}, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	component.config.Profiles["archive"] = ProfileConfig{
		MaxSize: 3 * 1024 * 1024 * 1024,
	}
	if got := component.maxTusSize(); got != 3*1024*1024*1024 {
		t.Fatalf("maxTusSize() = %d, want largest profile max size", got)
	}
}

func TestCleanupExpiredAbortsCompleteTusUploadID(t *testing.T) {
	store := &fakeMetadataStore{
		expiredTus: []ExpiredTusUpload{{
			SessionID:   3,
			FileID:      1,
			ReferenceID: 2,
			TusUploadID: "avatar/2026/06/26/file-test-1001.png+multipart",
			ObjectKey:   "files/avatar/2026/06/26/file-test-1001.png",
		}},
	}
	api := &fakeTusS3API{}
	component, err := New(store, &fakeObjectStore{}, fakeFlake(1001),
		WithBucket("egoadmin"),
		WithTusS3API(api),
		WithIDCodec(newTestIDCodec(t)),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cfg := component.Config()
	cfg.Tus.Enabled = true
	cfg.Tus.Path = "/tus/upload"
	cfg.Tus.ObjectPrefix = "files"
	cfg.Tus.MetadataPrefix = "tus-meta"

	server, err := newTusServer(component, MultipartOptions{})
	if err != nil {
		t.Fatalf("newTusServer() error = %v", err)
	}
	component.tus = server
	t.Cleanup(func() { _ = server.Close() })

	result, err := component.CleanupExpired(context.Background(), time.Now(), 100)
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if result.AbortedTusUploads != 1 {
		t.Fatalf("AbortedTusUploads = %d, want 1", result.AbortedTusUploads)
	}
	if len(store.abortedTusIDs) != 1 || store.abortedTusIDs[0] != 3 {
		t.Fatalf("aborted sessions = %#v, want session 3", store.abortedTusIDs)
	}
	if api.abortedKey != "files/avatar/2026/06/26/file-test-1001.png" || api.abortedUploadID != "multipart" {
		t.Fatalf("abort call key=%q uploadID=%q", api.abortedKey, api.abortedUploadID)
	}
}

func TestCleanupExpiredMarksTusUploadAbortedWithoutCompleteID(t *testing.T) {
	store := &fakeMetadataStore{
		expiredTus: []ExpiredTusUpload{{
			SessionID:   3,
			FileID:      1,
			ReferenceID: 2,
			TusUploadID: "avatar/2026/06/26/file-test-1001.png",
			ObjectKey:   "files/avatar/2026/06/26/file-test-1001.png",
		}},
	}
	component, err := New(store, &fakeObjectStore{}, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := component.CleanupExpired(context.Background(), time.Now(), 100)
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if result.AbortedTusUploads != 1 {
		t.Fatalf("AbortedTusUploads = %d, want 1", result.AbortedTusUploads)
	}
	if len(store.abortedTusIDs) != 1 || store.abortedTusIDs[0] != 3 {
		t.Fatalf("aborted sessions = %#v, want session 3", store.abortedTusIDs)
	}
}

func TestCleanupExpiredMarksCompleteTusUploadAbortedWithoutTusServer(t *testing.T) {
	store := &fakeMetadataStore{
		expiredTus: []ExpiredTusUpload{{
			SessionID:   3,
			FileID:      1,
			ReferenceID: 2,
			TusUploadID: "avatar/2026/06/26/file-test-1001.png+multipart",
			ObjectKey:   "files/avatar/2026/06/26/file-test-1001.png",
		}},
	}
	component, err := New(store, &fakeObjectStore{}, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := component.CleanupExpired(context.Background(), time.Now(), 100)
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if result.AbortedTusUploads != 1 {
		t.Fatalf("AbortedTusUploads = %d, want 1", result.AbortedTusUploads)
	}
	if len(store.abortedTusIDs) != 1 || store.abortedTusIDs[0] != 3 {
		t.Fatalf("aborted sessions = %#v, want session 3", store.abortedTusIDs)
	}
}

func tusHookEvent(uploadID string, objectKey string) tusd.HookEvent {
	return tusd.HookEvent{
		Context: context.Background(),
		Upload: tusd.FileInfo{
			ID:       uploadID,
			MetaData: tusd.MetaData{"objectKey": objectKey},
		},
	}
}

type fakeTusS3API struct {
	abortedKey      string
	abortedUploadID string
}

func (f *fakeTusS3API) PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeTusS3API) ListParts(context.Context, *s3.ListPartsInput, ...func(*s3.Options)) (*s3.ListPartsOutput, error) {
	return &s3.ListPartsOutput{}, nil
}

func (f *fakeTusS3API) UploadPart(context.Context, *s3.UploadPartInput, ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	return &s3.UploadPartOutput{ETag: aws.String("etag")}, nil
}

func (f *fakeTusS3API) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (f *fakeTusS3API) HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return &s3.HeadObjectOutput{ContentLength: aws.Int64(0)}, nil
}

func (f *fakeTusS3API) CreateMultipartUpload(context.Context, *s3.CreateMultipartUploadInput, ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	return &s3.CreateMultipartUploadOutput{UploadId: aws.String("multipart")}, nil
}

func (f *fakeTusS3API) AbortMultipartUpload(_ context.Context, input *s3.AbortMultipartUploadInput, _ ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	if f != nil && input != nil {
		if input.Key != nil {
			f.abortedKey = *input.Key
		}
		if input.UploadId != nil {
			f.abortedUploadID = *input.UploadId
		}
	}
	return &s3.AbortMultipartUploadOutput{}, nil
}

func (f *fakeTusS3API) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return &s3.DeleteObjectOutput{}, nil
}

func (f *fakeTusS3API) DeleteObjects(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	return &s3.DeleteObjectsOutput{}, nil
}

func (f *fakeTusS3API) CompleteMultipartUpload(context.Context, *s3.CompleteMultipartUploadInput, ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	return &s3.CompleteMultipartUploadOutput{}, nil
}

func (f *fakeTusS3API) UploadPartCopy(context.Context, *s3.UploadPartCopyInput, ...func(*s3.Options)) (*s3.UploadPartCopyOutput, error) {
	return &s3.UploadPartCopyOutput{CopyPartResult: &types.CopyPartResult{ETag: aws.String("etag"), LastModified: aws.Time(time.Now())}}, nil
}
