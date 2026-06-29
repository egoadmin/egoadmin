package cdn

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/idgen/idcodec"
	"github.com/egoadmin/egoadmin/internal/component/upload"
	"github.com/gin-gonic/gin"
)

func TestAccessSignature(t *testing.T) {
	now := time.Unix(1000, 0)
	expires := now.Add(time.Minute).Unix()
	token := signAccess("secret", "/cdn/file/ref-test", expires)
	if err := verifyAccessSignature("secret", "/cdn/file/ref-test", strconv.FormatInt(expires, 10), token, now); err != nil {
		t.Fatalf("verifyAccessSignature() error = %v", err)
	}
	if err := verifyAccessSignature("secret", "/cdn/file/ref-other", strconv.FormatInt(expires, 10), token, now); err != ErrSignatureInvalid {
		t.Fatalf("verifyAccessSignature(wrong material) = %v, want invalid", err)
	}
	if err := verifyAccessSignature("secret", "/cdn/file/ref-test", strconv.FormatInt(expires, 10), token, now.Add(2*time.Minute)); err != ErrSignatureExpired {
		t.Fatalf("verifyAccessSignature(expired) = %v, want expired", err)
	}
}

func TestEnsureDownloadable(t *testing.T) {
	now := time.Now()
	base := &upload.DownloadObject{
		ReferenceStatus: upload.ReferenceStatusBound,
		ObjectStatus:    upload.ObjectStatusAvailable,
		ExpiresAt:       now.Add(time.Hour),
	}
	if err := ensureDownloadable(base, now, false); err != nil {
		t.Fatalf("ensureDownloadable(bound) error = %v", err)
	}
	temporary := *base
	temporary.ReferenceStatus = upload.ReferenceStatusTemporary
	if err := ensureDownloadable(&temporary, now, false); err != ErrReferenceGone {
		t.Fatalf("ensureDownloadable(temp disallowed) = %v, want gone", err)
	}
	if err := ensureDownloadable(&temporary, now, true); err != nil {
		t.Fatalf("ensureDownloadable(temp allowed) error = %v", err)
	}
	released := *base
	released.ReferenceStatus = upload.ReferenceStatusReleased
	if err := ensureDownloadable(&released, now, true); err != ErrReferenceGone {
		t.Fatalf("ensureDownloadable(released) = %v, want gone", err)
	}
	unavailable := *base
	unavailable.ObjectStatus = upload.ObjectStatusDeleting
	if err := ensureDownloadable(&unavailable, now, true); err != ErrObjectUnavailable {
		t.Fatalf("ensureDownloadable(unavailable) = %v, want unavailable", err)
	}
	if status := statusFromError(ErrObjectUnavailable); status != http.StatusGone {
		t.Fatalf("statusFromError(unavailable) = %d, want gone", status)
	}
}

func TestFileHandlerServesContentDisposition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	component := newTestComponent(t, &fakeDownloadStore{
		object: &upload.DownloadObject{
			ReferenceID:     20001,
			FileID:          30001,
			ObjectKey:       "files/document/2026/06/26/file-test-30001.pdf",
			OriginalName:    "report.pdf",
			ContentType:     "application/pdf",
			Size:            int64(len("file body")),
			Profile:         "document",
			ReferenceStatus: upload.ReferenceStatusBound,
			ObjectStatus:    upload.ObjectStatusAvailable,
			AvailableAt:     ptrTime(time.Unix(1000, 0)),
		},
	}, &fakeObjectStore{data: []byte("file body"), contentType: "application/pdf"})
	handler := component.FileHandler(Options{
		BeforeFileHandle: func(*gin.Context) (*AuthContext, error) {
			return &AuthContext{UserID: 20001}, nil
		},
	})
	publicReferenceID := component.publicReferenceID(20001)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "referenceId", Value: publicReferenceID}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/cdn/file/"+publicReferenceID+"?display=inline", nil)

	handler(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "inline") || !strings.Contains(got, "report.pdf") {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if rec.Body.String() != "file body" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestFileHandlerAcceptsSignedURLWithDisplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	component := newTestComponent(t, &fakeDownloadStore{
		object: &upload.DownloadObject{
			ReferenceID:     20001,
			FileID:          30001,
			ObjectKey:       "files/document/2026/06/26/file-test-30001.pdf",
			OriginalName:    "report.pdf",
			ContentType:     "application/pdf",
			Size:            int64(len("file body")),
			Profile:         "document",
			ReferenceStatus: upload.ReferenceStatusBound,
			ObjectStatus:    upload.ObjectStatusAvailable,
			AvailableAt:     ptrTime(time.Unix(1000, 0)),
		},
	}, &fakeObjectStore{data: []byte("file body"), contentType: "application/pdf"})
	handler := component.FileHandler(Options{})
	signedPath := component.SignedFileURL(20001, "inline", time.Now())
	publicReferenceID := component.publicReferenceID(20001)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "referenceId", Value: publicReferenceID}}
	ctx.Request = httptest.NewRequest(http.MethodGet, signedPath, nil)

	handler(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q signedPath=%q", rec.Code, rec.Body.String(), signedPath)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "inline") {
		t.Fatalf("Content-Disposition = %q, want inline", got)
	}
}

func TestFileHandlerRejectsRawNumericReferenceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	component := newTestComponent(t, &fakeDownloadStore{}, &fakeObjectStore{})
	handler := component.FileHandler(Options{})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "referenceId", Value: "20001"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/cdn/file/20001", nil)

	handler(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %q, want bad request", rec.Code, rec.Body.String())
	}
}

func TestFileHandlerRejectsSignedURLWithTamperedDisplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	component := newTestComponent(t, &fakeDownloadStore{
		object: &upload.DownloadObject{
			ReferenceID:     20001,
			FileID:          30001,
			ObjectKey:       "files/document/2026/06/26/file-test-30001.pdf",
			OriginalName:    "report.pdf",
			ContentType:     "application/pdf",
			Size:            int64(len("file body")),
			Profile:         "document",
			ReferenceStatus: upload.ReferenceStatusBound,
			ObjectStatus:    upload.ObjectStatusAvailable,
		},
	}, &fakeObjectStore{data: []byte("file body"), contentType: "application/pdf"})
	handler := component.FileHandler(Options{})
	signedPath := component.SignedFileURL(20001, "attachment", time.Now())
	parsed, err := url.Parse(signedPath)
	if err != nil {
		t.Fatalf("parse signed URL: %v", err)
	}
	values := parsed.Query()
	values.Set("display", "inline")
	parsed.RawQuery = values.Encode()

	publicReferenceID := component.publicReferenceID(20001)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "referenceId", Value: publicReferenceID}}
	ctx.Request = httptest.NewRequest(http.MethodGet, parsed.String(), nil)

	handler(ctx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %q, want forbidden", rec.Code, rec.Body.String())
	}
}

func TestImageHandlerRejectsInvalidProcessPath(t *testing.T) {
	component := newTestComponent(t, &fakeDownloadStore{}, &fakeObjectStore{})
	_, err := component.cleanProcessPath("../300x200")
	if err != ErrInvalidProcessPath {
		t.Fatalf("cleanProcessPath() = %q, want invalid", err)
	}
	_, err = component.cleanProcessPath("300x200/smart")
	if err != nil {
		t.Fatalf("cleanProcessPath(valid) error = %v", err)
	}
}

func TestImageHandlerProxiesSignedProcessorPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var requestedPath string
	processor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("image body"))
	}))
	defer processor.Close()

	store := &fakeDownloadStore{
		object: &upload.DownloadObject{
			ReferenceID:     20001,
			FileID:          30001,
			ObjectKey:       "files/avatar/2026/06/26/file-test-30001.png",
			OriginalName:    "avatar.png",
			ContentType:     "image/png",
			Size:            10,
			Profile:         "avatar",
			ReferenceStatus: upload.ReferenceStatusBound,
			ObjectStatus:    upload.ObjectStatusAvailable,
		},
	}
	component := newTestComponent(t, store, &fakeObjectStore{},
		WithImageProcessorConfig(&ImageProcessorConfig{URL: processor.URL, Secret: "processor-secret", Timeout: time.Second}),
	)
	handler := component.ImageHandler()
	publicReferenceID := component.publicReferenceID(20001)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "referenceId", Value: publicReferenceID}, {Key: "processPath", Value: "/300x200/smart"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/cdn/image/"+publicReferenceID+"/300x200/smart", nil)

	handler(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
	}
	if !strings.HasSuffix(requestedPath, "/300x200/smart/files/avatar/2026/06/26/file-test-30001.png") {
		t.Fatalf("processor path = %q, want signed processor path", requestedPath)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("Content-Type = %q, want image/webp", got)
	}
	if rec.Body.String() != "image body" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestImageHandlerRejectsInvalidSignatureEvenWhenPublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeDownloadStore{
		object: &upload.DownloadObject{
			ReferenceID:     20001,
			FileID:          30001,
			ObjectKey:       "files/avatar/2026/06/26/file-test-30001.png",
			OriginalName:    "avatar.png",
			ContentType:     "image/png",
			Size:            10,
			Profile:         "avatar",
			ReferenceStatus: upload.ReferenceStatusBound,
			ObjectStatus:    upload.ObjectStatusAvailable,
		},
	}
	component := newTestComponent(t, store, &fakeObjectStore{})
	handler := component.ImageHandler()
	publicReferenceID := component.publicReferenceID(20001)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "referenceId", Value: publicReferenceID}, {Key: "processPath", Value: "/300x200/smart"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/cdn/image/"+publicReferenceID+"/300x200/smart?expires=9999999999&token=bad", nil)

	handler(ctx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %q, want forbidden", rec.Code, rec.Body.String())
	}
}

func newTestComponent(t *testing.T, metadata upload.MetadataStore, object upload.ObjectStore, opts ...Option) *Component {
	t.Helper()
	uploadComponent, err := upload.New(metadata, object, fakeFlake(1), upload.WithIDCodec(newTestIDCodec(t)))
	if err != nil {
		t.Fatalf("upload.New() error = %v", err)
	}
	opts = append([]Option{
		WithConfig(&Config{
			FilePath:            "/cdn/file",
			ImagePath:           "/cdn/image",
			SignSecret:          "cdn-secret",
			DefaultSignedTTL:    time.Minute,
			PublicImage:         true,
			AllowTemporaryImage: false,
			MaxProcessPathBytes: 2048,
			MaxQueryBytes:       2048,
		}),
	}, opts...)
	component, err := New(uploadComponent, object, opts...)
	if err != nil {
		t.Fatalf("cdn.New() error = %v", err)
	}
	return component
}

func newTestIDCodec(t *testing.T) *idcodec.Component {
	t.Helper()
	cfg := idcodec.DefaultConfig()
	cfg.Secret = "0123456789abcdef0123456789abcdef"
	cfg.EnableMetrics = false
	return idcodec.DefaultContainer().Build(idcodec.WithConfig(cfg), idcodec.WithName("component.idgen.codec.cdn.test"))
}

type fakeFlake uint64

func (f fakeFlake) Get() (uint64, error) {
	return uint64(f), nil
}

type fakeDownloadStore struct {
	object *upload.DownloadObject
}

func (s *fakeDownloadStore) CreateMultipart(context.Context, upload.CreateMultipartCommand) (*upload.CreateMultipartResult, error) {
	return nil, nil
}

func (s *fakeDownloadStore) CreateTus(context.Context, upload.CreateTusCommand) (*upload.CreateTusResult, error) {
	return nil, nil
}

func (s *fakeDownloadStore) CreateInstantReference(context.Context, upload.CreateInstantReferenceCommand) (*upload.CreateInstantReferenceResult, error) {
	return nil, nil
}

func (s *fakeDownloadStore) CommitReference(context.Context, upload.CommitReferenceCommand) (*upload.ReferenceDetail, error) {
	return nil, nil
}

func (s *fakeDownloadStore) ReleasePreviousReferences(context.Context, upload.ReleasePreviousReferencesCommand) error {
	return nil
}

func (s *fakeDownloadStore) GetReference(context.Context, uint64, uint64) (*upload.ReferenceDetail, error) {
	return nil, nil
}

func (s *fakeDownloadStore) GetBoundReference(context.Context, upload.GetBoundReferenceCommand) (*upload.ReferenceDetail, error) {
	return nil, upload.ErrReferenceNotFound
}

func (s *fakeDownloadStore) GetDownloadReference(context.Context, uint64) (*upload.DownloadObject, error) {
	if s.object == nil {
		return nil, upload.ErrReferenceNotFound
	}
	return s.object, nil
}

func (s *fakeDownloadStore) GetDownloadReferenceForOwner(context.Context, uint64, uint64) (*upload.DownloadObject, error) {
	return s.GetDownloadReference(context.Background(), 0)
}

func (s *fakeDownloadStore) ReleaseReference(context.Context, upload.ReleaseReferenceCommand) error {
	return nil
}

func (s *fakeDownloadStore) ExpireTemporaryReferences(context.Context, time.Time, int) ([]upload.ExpiredReference, error) {
	return nil, nil
}

func (s *fakeDownloadStore) FindUnreferencedObjects(context.Context, int) ([]upload.UnreferencedObject, error) {
	return nil, nil
}

func (s *fakeDownloadStore) MarkObjectDeleting(context.Context, uint64) (bool, error) {
	return false, nil
}
func (s *fakeDownloadStore) MarkObjectDeleted(context.Context, uint64) error { return nil }
func (s *fakeDownloadStore) FindReusableObject(context.Context, upload.FindReusableObjectCommand) (*upload.ReusableObject, error) {
	return nil, nil
}
func (s *fakeDownloadStore) MarkObjectAvailable(context.Context, uint64) error { return nil }
func (s *fakeDownloadStore) MarkSessionFinished(context.Context, uint64) error { return nil }
func (s *fakeDownloadStore) MarkUploadFailed(context.Context, uint64) error    { return nil }
func (s *fakeDownloadStore) MarkTusUploadFinished(context.Context, upload.MarkTusUploadFinishedCommand) (*upload.TusUploadDetail, error) {
	return nil, nil
}

func (s *fakeDownloadStore) MarkTusUploadCreated(context.Context, upload.MarkTusUploadCreatedCommand) error {
	return nil
}

func (s *fakeDownloadStore) FindExpiredTusUploads(context.Context, time.Time, int) ([]upload.ExpiredTusUpload, error) {
	return nil, nil
}
func (s *fakeDownloadStore) MarkTusUploadAborted(context.Context, uint64) error { return nil }
func (s *fakeDownloadStore) FindTusMetadataForCleanup(context.Context, int) ([]upload.TusMetadataObject, error) {
	return nil, nil
}

func (s *fakeDownloadStore) MarkTusMetadataCleaning(context.Context, uint64) (bool, error) {
	return false, nil
}
func (s *fakeDownloadStore) MarkTusMetadataCleaned(context.Context, uint64) error { return nil }

type fakeObjectStore struct {
	data        []byte
	contentType string
}

func (s *fakeObjectStore) Put(context.Context, string, io.Reader, int64, upload.PutOptions) error {
	return nil
}

func (s *fakeObjectStore) Get(_ context.Context, key string) (upload.ObjectReader, error) {
	return &fakeObjectReader{
		Reader: bytes.NewReader(s.data),
		info: upload.ObjectInfo{
			Key:         key,
			Size:        int64(len(s.data)),
			ContentType: s.contentType,
		},
	}, nil
}
func (s *fakeObjectStore) Delete(context.Context, string) error { return nil }
func (s *fakeObjectStore) Stat(context.Context, string) (upload.ObjectInfo, error) {
	return upload.ObjectInfo{}, nil
}

type fakeObjectReader struct {
	*bytes.Reader
	info upload.ObjectInfo
}

func (r *fakeObjectReader) Close() error { return nil }
func (r *fakeObjectReader) Stat() (upload.ObjectInfo, error) {
	return r.info, nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
