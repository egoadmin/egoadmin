package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/idgen/idcodec"
	"github.com/gin-gonic/gin"
)

const testIDCodecSecret = "0123456789abcdef0123456789abcdef"

func newTestIDCodec(t *testing.T) *idcodec.Component {
	t.Helper()
	cfg := idcodec.DefaultConfig()
	cfg.Secret = testIDCodecSecret
	cfg.EnableMetrics = false
	return idcodec.DefaultContainer().Build(idcodec.WithConfig(cfg), idcodec.WithName("component.idgen.codec.upload.test"))
}

func TestConfigNormalizeResolvesProfiles(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	avatar := cfg.Profile("avatar")
	if avatar.MaxSize != 5*1024*1024 {
		t.Fatalf("avatar max size = %d", avatar.MaxSize)
	}
	if !contains(avatar.AllowedExtensions, "png") {
		t.Fatalf("avatar extensions = %v, want inherited image extensions", avatar.AllowedExtensions)
	}
}

func TestUploadMultipartCreatesLogicalReference(t *testing.T) {
	store := &fakeMetadataStore{}
	object := &fakeObjectStore{}
	component, err := New(store, object, fakeFlake(1001),
		WithBucket("egoadmin"),
		WithBaseURL("http://127.0.0.1:9000/egoadmin/"),
		WithIDCodec(newTestIDCodec(t)),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := component.UploadMultipart(context.Background(), MultipartCommand{
		Profile:      "avatar",
		OwnerUserID:  20001,
		OriginalName: "avatar.PNG",
		ContentType:  "image/png",
		Size:         12,
		Reader:       strings.NewReader("hello upload"),
	})
	if err != nil {
		t.Fatalf("UploadMultipart() error = %v", err)
	}
	if result.FileID != 1 || result.ReferenceID != 2 {
		t.Fatalf("result ids = %d/%d, want 1/2", result.FileID, result.ReferenceID)
	}
	if !strings.HasPrefix(result.ObjectKey, "files/avatar/") {
		t.Fatalf("object key = %q, want logical files/avatar prefix", result.ObjectKey)
	}
	if !strings.Contains(result.ObjectKey, "/file-") || !strings.HasSuffix(result.ObjectKey, ".png") {
		t.Fatalf("object key = %q, want public file id object name", result.ObjectKey)
	}
	if object.key != result.ObjectKey || object.size != 12 || object.contentType != "image/png" {
		t.Fatalf("object put = key %q size %d content-type %q", object.key, object.size, object.contentType)
	}
	if store.availableID != result.FileID || store.finishedID != 3 {
		t.Fatalf("store available/finished = %d/%d", store.availableID, store.finishedID)
	}
	publicReferenceID := component.PublicReferenceID(result.ReferenceID)
	if publicReferenceID == "" || publicReferenceID == "2" {
		t.Fatalf("public reference id = %q, want encoded id", publicReferenceID)
	}
	if result.URL != "/cdn/image/"+publicReferenceID {
		t.Fatalf("url = %q, want cdn image reference URL", result.URL)
	}
}

func TestUploadMultipartMarksFailedOnPutError(t *testing.T) {
	store := &fakeMetadataStore{}
	object := &fakeObjectStore{err: errors.New("put failed")}
	component, err := New(store, object, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = component.UploadMultipart(context.Background(), MultipartCommand{
		Profile:      "avatar",
		OwnerUserID:  20001,
		OriginalName: "avatar.png",
		ContentType:  "image/png",
		Size:         12,
		Reader:       strings.NewReader("hello upload"),
	})
	if err == nil {
		t.Fatalf("UploadMultipart() error = nil, want put error")
	}
	if store.failedID != 3 {
		t.Fatalf("failed session = %d, want 3", store.failedID)
	}
}

func TestUploadMultipartRejectsUnknownProfile(t *testing.T) {
	component, err := New(&fakeMetadataStore{}, &fakeObjectStore{}, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = component.UploadMultipart(context.Background(), MultipartCommand{
		Profile:      "not-exists",
		OwnerUserID:  20001,
		OriginalName: "avatar.png",
		ContentType:  "image/png",
		Size:         12,
		Reader:       strings.NewReader("hello upload"),
	})
	if err == nil {
		t.Fatalf("UploadMultipart() error = nil, want unknown profile error")
	}
}

func TestUploadMultipartRejectsTusRequiredProfile(t *testing.T) {
	component, err := New(&fakeMetadataStore{}, &fakeObjectStore{}, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = component.UploadMultipart(context.Background(), MultipartCommand{
		Profile:      "video",
		OwnerUserID:  20001,
		OriginalName: "movie.mp4",
		ContentType:  "video/mp4",
		Size:         12,
		Reader:       strings.NewReader("hello upload"),
	})
	if err == nil {
		t.Fatalf("UploadMultipart() error = nil, want tus-required rejection")
	}
}

func TestProfilesAreStableAndInstantUpload(t *testing.T) {
	store := &fakeMetadataStore{reusable: &ReusableObject{FileID: 9, ObjectKey: "files/document/a.pdf"}}
	component, err := New(store, &fakeObjectStore{}, fakeFlake(1001), WithBaseURL("http://files/"), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	profiles := component.Profiles()
	if len(profiles) == 0 || profiles[0].Name == "" {
		t.Fatalf("profiles = %#v, want stable profile list", profiles)
	}
	for i := 1; i < len(profiles); i++ {
		if profiles[i-1].Name > profiles[i].Name {
			t.Fatalf("profiles not sorted: %#v", profiles)
		}
	}
	result, err := component.InstantUpload(context.Background(), InstantCommand{
		Profile:      "document",
		OwnerUserID:  20001,
		SHA256:       "abc",
		Size:         12,
		OriginalName: "doc.pdf",
	})
	if err != nil {
		t.Fatalf("InstantUpload() error = %v", err)
	}
	publicReferenceID := component.PublicReferenceID(10)
	if !result.Hit || result.ShouldUpload || result.ReferenceID != publicReferenceID {
		t.Fatalf("instant result = %#v", result)
	}
	if result.URL != "/cdn/file/"+publicReferenceID {
		t.Fatalf("instant url = %q, want cdn file reference URL", result.URL)
	}
}

func TestProfilesHandlerRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	component, err := New(&fakeMetadataStore{}, &fakeObjectStore{}, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := component.ProfilesHandler(MultipartOptions{
		BeforeHandle: func(*gin.Context) (*AuthContext, error) {
			return nil, errors.New("missing auth")
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/upload/profiles", nil)
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req

	handler(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want http 200 ego error envelope", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ecode.v1.ERROR_FAILED") {
		t.Fatalf("body = %q, want upload error envelope", rec.Body.String())
	}
}

func TestCleanupExpiredDeletesUnreferencedObjects(t *testing.T) {
	store := &fakeMetadataStore{
		expired: []ExpiredReference{{ReferenceID: 11, FileID: 1}},
		unref:   []UnreferencedObject{{FileID: 1, ObjectKey: "files/avatar/2026/06/26/file-test-1.png"}},
	}
	object := &fakeObjectStore{}
	component, err := New(store, object, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := component.CleanupExpired(context.Background(), time.Now(), 100)
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if result.ExpiredReferences != 1 || result.DeletedObjects != 1 {
		t.Fatalf("cleanup result = %#v, want 1 expired and 1 deleted", result)
	}
	if len(store.deleting) != 1 || store.deleting[0] != 1 {
		t.Fatalf("deleting = %#v, want file 1", store.deleting)
	}
	if len(store.deleted) != 1 || store.deleted[0] != 1 {
		t.Fatalf("deleted = %#v, want file 1", store.deleted)
	}
	if object.deletedKey != "files/avatar/2026/06/26/file-test-1.png" {
		t.Fatalf("deleted object = %q", object.deletedKey)
	}
}

func TestCleanupExpiredSkipsObjectWhenStoreCannotMarkDeleting(t *testing.T) {
	store := &fakeMetadataStore{
		unref:            []UnreferencedObject{{FileID: 1, ObjectKey: "files/avatar/2026/06/26/file-test-1.png"}},
		skipMarkDeleting: true,
	}
	object := &fakeObjectStore{}
	component, err := New(store, object, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := component.CleanupExpired(context.Background(), time.Now(), 100)
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if result.DeletedObjects != 0 {
		t.Fatalf("deleted objects = %d, want 0", result.DeletedObjects)
	}
	if object.deletedKey != "" {
		t.Fatalf("deleted object = %q, want none", object.deletedKey)
	}
}

func TestCreateTusUploadAndCleanupMetadata(t *testing.T) {
	store := &fakeMetadataStore{
		tusMetadata: []TusMetadataObject{{
			SessionID:  3,
			FileID:     1,
			TusInfoKey: "tus-meta/avatar/2026/06/26/file-test-1001.png.info",
			TusPartKey: "tus-meta/avatar/2026/06/26/file-test-1001.png.part",
		}},
	}
	object := &fakeObjectStore{}
	component, err := New(store, object, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	created, err := component.CreateTusUpload(context.Background(), TusCreateCommand{
		Profile:      "avatar",
		OwnerUserID:  20001,
		OriginalName: "avatar.png",
		ContentType:  "image/png",
		Size:         12,
	})
	if err != nil {
		t.Fatalf("CreateTusUpload() error = %v", err)
	}
	if created.TusUploadID == created.ObjectKey || !strings.HasPrefix(created.TusUploadID, "avatar/") || !strings.HasPrefix(created.TusInfoKey, "tus-meta/") {
		t.Fatalf("created tus = %#v, want logical upload id and metadata key", created)
	}

	result, err := component.CleanupExpired(context.Background(), time.Now(), 100)
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}
	if result.CleanedTusMetadata != 1 {
		t.Fatalf("CleanedTusMetadata = %d, want 1", result.CleanedTusMetadata)
	}
	if len(object.deletedKeys) != 2 || object.deletedKeys[0] != store.tusMetadata[0].TusInfoKey || object.deletedKeys[1] != store.tusMetadata[0].TusPartKey {
		t.Fatalf("deleted metadata keys = %#v", object.deletedKeys)
	}
	if store.metadataCleanedID != 3 {
		t.Fatalf("metadataCleanedID = %d, want 3", store.metadataCleanedID)
	}
}

func TestCleanupTusMetadataRejectsUnexpectedPrefix(t *testing.T) {
	store := &fakeMetadataStore{
		tusMetadata: []TusMetadataObject{{
			SessionID:  3,
			FileID:     1,
			TusInfoKey: "files/avatar/2026/06/26/file-test-1001.png.info",
		}},
	}
	component, err := New(store, &fakeObjectStore{}, fakeFlake(1001), WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = component.CleanupExpired(context.Background(), time.Now(), 100)
	if err == nil {
		t.Fatalf("CleanupExpired() error = nil, want unexpected prefix error")
	}
}

type fakeFlake uint64

func (f fakeFlake) Get() (uint64, error) {
	return uint64(f), nil
}

type fakeMetadataStore struct {
	mu                sync.Mutex
	availableID       uint64
	finishedID        uint64
	failedID          uint64
	reusable          *ReusableObject
	expired           []ExpiredReference
	unref             []UnreferencedObject
	expiredTus        []ExpiredTusUpload
	deleting          []uint64
	deleted           []uint64
	tusMetadata       []TusMetadataObject
	metadataCleaning  []uint64
	metadataCleanedID uint64
	createdTus        *CreateTusCommand
	tusCreated        *MarkTusUploadCreatedCommand
	finishedTus       *MarkTusUploadFinishedCommand
	abortedTusIDs     []uint64
	skipMarkDeleting  bool
}

func (s *fakeMetadataStore) CreateMultipart(ctx context.Context, cmd CreateMultipartCommand) (*CreateMultipartResult, error) {
	return &CreateMultipartResult{
		FileID:      1,
		ReferenceID: 2,
		SessionID:   3,
		ExpiresAt:   time.Now().Add(time.Hour),
	}, nil
}

func (s *fakeMetadataStore) CreateTus(ctx context.Context, cmd CreateTusCommand) (*CreateTusResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := cmd
	s.createdTus = &cp
	return &CreateTusResult{
		FileID:      1,
		ReferenceID: 2,
		SessionID:   3,
		ObjectKey:   cmd.ObjectKey,
		TusUploadID: cmd.TusUploadID,
		TusInfoKey:  cmd.TusInfoKey,
		TusPartKey:  cmd.TusPartKey,
		ExpiresAt:   cmd.ExpiresAt,
	}, nil
}

func (s *fakeMetadataStore) MarkObjectAvailable(ctx context.Context, fileID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.availableID = fileID
	return nil
}

func (s *fakeMetadataStore) MarkSessionFinished(ctx context.Context, sessionID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishedID = sessionID
	return nil
}

func (s *fakeMetadataStore) MarkUploadFailed(ctx context.Context, sessionID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedID = sessionID
	return nil
}

func (s *fakeMetadataStore) CreateInstantReference(ctx context.Context, cmd CreateInstantReferenceCommand) (*CreateInstantReferenceResult, error) {
	return &CreateInstantReferenceResult{
		FileID:      cmd.FileID,
		ReferenceID: 10,
		ObjectKey:   s.reusable.ObjectKey,
		ExpiresAt:   cmd.ExpiresAt,
	}, nil
}

func (s *fakeMetadataStore) CommitReference(ctx context.Context, cmd CommitReferenceCommand) (*ReferenceDetail, error) {
	return &ReferenceDetail{
		ReferenceID: cmd.ReferenceID,
		OwnerUserID: cmd.OwnerUserID,
		Profile:     cmd.Profile,
		Status:      ReferenceStatusBound,
		ObjectKey:   "files/avatar/2026/06/26/file-test-1.png",
		ExpiresAt:   time.Now().Add(time.Hour),
	}, nil
}

func (s *fakeMetadataStore) ReleasePreviousReferences(ctx context.Context, cmd ReleasePreviousReferencesCommand) error {
	return nil
}

func (s *fakeMetadataStore) GetReference(ctx context.Context, referenceID uint64, ownerUserID uint64) (*ReferenceDetail, error) {
	return &ReferenceDetail{
		ReferenceID: referenceID,
		OwnerUserID: ownerUserID,
		Profile:     "avatar",
		Status:      ReferenceStatusTemporary,
		ObjectKey:   "files/avatar/2026/06/26/file-test-1.png",
		ExpiresAt:   time.Now().Add(time.Hour),
	}, nil
}

func (s *fakeMetadataStore) GetBoundReference(context.Context, GetBoundReferenceCommand) (*ReferenceDetail, error) {
	return nil, ErrReferenceNotFound
}

func (s *fakeMetadataStore) GetDownloadReference(ctx context.Context, referenceID uint64) (*DownloadObject, error) {
	return s.downloadReference(referenceID, 0), nil
}

func (s *fakeMetadataStore) GetDownloadReferenceForOwner(ctx context.Context, referenceID uint64, ownerUserID uint64) (*DownloadObject, error) {
	return s.downloadReference(referenceID, ownerUserID), nil
}

func (s *fakeMetadataStore) downloadReference(referenceID uint64, ownerUserID uint64) *DownloadObject {
	return &DownloadObject{
		ReferenceID:     referenceID,
		FileID:          1,
		OwnerUserID:     ownerUserID,
		Bucket:          "egoadmin",
		ObjectKey:       "files/avatar/2026/06/26/file-test-1.png",
		OriginalName:    "avatar.png",
		ContentType:     "image/png",
		Size:            12,
		Profile:         "avatar",
		ReferenceStatus: ReferenceStatusTemporary,
		ObjectStatus:    ObjectStatusAvailable,
		ExpiresAt:       time.Now().Add(time.Hour),
	}
}

func (s *fakeMetadataStore) ReleaseReference(ctx context.Context, cmd ReleaseReferenceCommand) error {
	return nil
}

func (s *fakeMetadataStore) ExpireTemporaryReferences(ctx context.Context, now time.Time, limit int) ([]ExpiredReference, error) {
	return s.expired, nil
}

func (s *fakeMetadataStore) FindUnreferencedObjects(ctx context.Context, limit int) ([]UnreferencedObject, error) {
	return s.unref, nil
}

func (s *fakeMetadataStore) MarkObjectDeleting(ctx context.Context, fileID uint64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.skipMarkDeleting {
		return false, nil
	}
	s.deleting = append(s.deleting, fileID)
	return true, nil
}

func (s *fakeMetadataStore) MarkObjectDeleted(ctx context.Context, fileID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, fileID)
	return nil
}

func (s *fakeMetadataStore) FindReusableObject(ctx context.Context, cmd FindReusableObjectCommand) (*ReusableObject, error) {
	return s.reusable, nil
}

func (s *fakeMetadataStore) MarkTusUploadFinished(ctx context.Context, cmd MarkTusUploadFinishedCommand) (*TusUploadDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := cmd
	s.finishedTus = &cp
	return &TusUploadDetail{SessionID: 3, FileID: 1, ReferenceID: 2, ObjectKey: cmd.ObjectKey}, nil
}

func (s *fakeMetadataStore) MarkTusUploadCreated(ctx context.Context, cmd MarkTusUploadCreatedCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := cmd
	s.tusCreated = &cp
	return nil
}

func (s *fakeMetadataStore) FindExpiredTusUploads(ctx context.Context, now time.Time, limit int) ([]ExpiredTusUpload, error) {
	return s.expiredTus, nil
}

func (s *fakeMetadataStore) MarkTusUploadAborted(ctx context.Context, sessionID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.abortedTusIDs = append(s.abortedTusIDs, sessionID)
	return nil
}

func (s *fakeMetadataStore) FindTusMetadataForCleanup(ctx context.Context, limit int) ([]TusMetadataObject, error) {
	return s.tusMetadata, nil
}

func (s *fakeMetadataStore) MarkTusMetadataCleaning(ctx context.Context, sessionID uint64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadataCleaning = append(s.metadataCleaning, sessionID)
	return true, nil
}

func (s *fakeMetadataStore) MarkTusMetadataCleaned(ctx context.Context, sessionID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadataCleanedID = sessionID
	return nil
}

func (s *fakeMetadataStore) currentFinishedTus() *MarkTusUploadFinishedCommand {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finishedTus
}

func (s *fakeMetadataStore) currentTusCreated() *MarkTusUploadCreatedCommand {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tusCreated
}

type fakeObjectStore struct {
	key         string
	size        int64
	contentType string
	err         error
	deletedKey  string
	deletedKeys []string
}

type fakeObjectReader struct {
	*bytes.Reader
	info ObjectInfo
}

func (s *fakeObjectStore) Put(ctx context.Context, key string, reader io.Reader, size int64, opts PutOptions) error {
	s.key = key
	s.size = size
	s.contentType = opts.ContentType
	return s.err
}

func (s *fakeObjectStore) Get(ctx context.Context, key string) (ObjectReader, error) {
	return &fakeObjectReader{
		Reader: bytes.NewReader([]byte("hello upload")),
		info:   ObjectInfo{Key: key, Size: int64(len("hello upload")), ContentType: s.contentType},
	}, nil
}

func (s *fakeObjectStore) Delete(ctx context.Context, key string) error {
	s.deletedKey = key
	s.deletedKeys = append(s.deletedKeys, key)
	return nil
}

func (s *fakeObjectStore) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	return ObjectInfo{Key: key, Size: s.size, ContentType: s.contentType}, nil
}

func (r *fakeObjectReader) Close() error {
	return nil
}

func (r *fakeObjectReader) Stat() (ObjectInfo, error) {
	return r.info, nil
}
