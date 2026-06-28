package upload

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egin"
	tusd "github.com/tus/tusd/v2/pkg/handler"
	"github.com/tus/tusd/v2/pkg/memorylocker"
	tuss3store "github.com/tus/tusd/v2/pkg/s3store"
)

type TusServer struct {
	component *Component
	handler   *tusd.Handler
	store     *tuss3store.S3Store
	opts      MultipartOptions
	stop      chan struct{}
	done      chan struct{}
	once      sync.Once
}

func RegisterTusRoutes(cc *egin.Component, component *Component, opts MultipartOptions) error {
	if !component.config.Tus.Enabled {
		return nil
	}
	if component.tusS3 == nil {
		return fmt.Errorf("upload: tus s3 api is required when tus is enabled")
	}
	server, err := newTusServer(component, opts)
	if err != nil {
		return err
	}
	component.tus = server
	cc.Any(component.config.Tus.Path, server.handle)
	cc.Any(component.config.Tus.Path+"/*path", server.handle)
	return nil
}

func newTusServer(component *Component, opts MultipartOptions) (*TusServer, error) {
	store := tuss3store.New(component.bucket, component.tusS3)
	store.ObjectPrefix = strings.Trim(component.config.Tus.ObjectPrefix, "/")
	store.MetadataObjectPrefix = strings.Trim(component.config.Tus.MetadataPrefix, "/")
	store.PreferredPartSize = component.config.Tus.PartSize
	store.MaxBufferedParts = component.config.Tus.MaxBufferedParts
	store.TemporaryDirectory = component.config.Tus.TemporaryDirectory
	store.SetConcurrentPartUploads(component.config.Tus.MaxConcurrentUploads)

	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker := memorylocker.New()
	locker.UseIn(composer)

	server := &TusServer{
		component: component,
		store:     &store,
		opts:      opts,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:                component.config.Tus.Path,
		StoreComposer:           composer,
		NotifyCompleteUploads:   true,
		NotifyCreatedUploads:    true,
		RespectForwardedHeaders: true,
		DisableDownload:         true,
		MaxSize:                 component.maxTusSize(),
		PreUploadCreateCallback: server.preCreate(opts),
	})
	if err != nil {
		return nil, err
	}
	server.handler = handler
	go server.consumeEvents()
	return server, nil
}

func (s *TusServer) preCreate(opts MultipartOptions) func(tusd.HookEvent) (tusd.HTTPResponse, tusd.FileInfoChanges, error) {
	return func(event tusd.HookEvent) (tusd.HTTPResponse, tusd.FileInfoChanges, error) {
		if err := s.component.CheckTusLocalTempLimit(event.Context); err != nil {
			return tusd.HTTPResponse{StatusCode: http.StatusTooManyRequests}, tusd.FileInfoChanges{}, err
		}
		auth, err := authFromTusEvent(event, opts)
		if err != nil {
			return tusd.HTTPResponse{StatusCode: http.StatusUnauthorized}, tusd.FileInfoChanges{}, err
		}
		filename := event.Upload.MetaData["filename"]
		contentType := firstNonEmpty(event.Upload.MetaData["filetype"], event.Upload.MetaData["contentType"])
		created, err := s.component.CreateTusUpload(event.Context, TusCreateCommand{
			Profile:      event.Upload.MetaData["profile"],
			OwnerUserID:  auth.UserID,
			OriginalName: filename,
			ContentType:  contentType,
			SHA256:       event.Upload.MetaData["sha256"],
			Size:         event.Upload.Size,
		})
		if err != nil {
			return tusd.HTTPResponse{StatusCode: http.StatusBadRequest}, tusd.FileInfoChanges{}, err
		}
		metadata := tusd.MetaData{}
		for key, value := range event.Upload.MetaData {
			metadata[key] = value
		}
		metadata["profile"] = createdObjectProfile(metadata["profile"])
		metadata["objectKey"] = created.ObjectKey
		publicFileID, err := s.component.publicID(publicFileIDPrefix, created.FileID)
		if err != nil {
			return tusd.HTTPResponse{StatusCode: http.StatusInternalServerError}, tusd.FileInfoChanges{}, err
		}
		publicReferenceID, err := s.component.publicID(publicReferenceIDPrefix, created.ReferenceID)
		if err != nil {
			return tusd.HTTPResponse{StatusCode: http.StatusInternalServerError}, tusd.FileInfoChanges{}, err
		}
		accessURL, err := s.component.mustAccessURL(createdObjectProfile(metadata["profile"]), created.ReferenceID)
		if err != nil {
			return tusd.HTTPResponse{StatusCode: http.StatusInternalServerError}, tusd.FileInfoChanges{}, err
		}
		return tusd.HTTPResponse{Header: tusd.HTTPHeader{
				"X-Upload-File-Id":      publicFileID,
				"X-Upload-Reference-Id": publicReferenceID,
				"X-Upload-Profile":      createdObjectProfile(metadata["profile"]),
				"X-Upload-Expires-At":   created.ExpiresAt.Format(time.RFC3339),
				"X-Upload-Status":       ReferenceStatusTemporary,
				"X-Upload-Url":          accessURL,
			}}, tusd.FileInfoChanges{
				ID:       created.TusUploadID,
				MetaData: metadata,
			}, nil
	}
}

func authFromTusEvent(event tusd.HookEvent, opts MultipartOptions) (*AuthContext, error) {
	return authFromTusRequest(event.Context, event.HTTPRequest.Header, opts)
}

func authFromTusRequest(ctx context.Context, header http.Header, opts MultipartOptions) (*AuthContext, error) {
	if opts.BeforeTusHandle == nil {
		return &AuthContext{}, nil
	}
	return opts.BeforeTusHandle(ctx, header)
}

func createdObjectProfile(profile string) string {
	if profile == "" {
		return DefaultProfile
	}
	return profile
}

func (s *TusServer) consumeEvents() {
	defer close(s.done)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case event := <-s.handler.CreatedUploads:
			if err := s.component.store.MarkTusUploadCreated(event.Context, MarkTusUploadCreatedCommand{
				TusUploadID: event.Upload.ID,
				ObjectKey:   event.Upload.MetaData["objectKey"],
			}); err != nil {
				elog.Error("record created tus upload metadata failed", elog.String("upload_id", event.Upload.ID), elog.FieldErr(err))
			}
		case event := <-s.handler.CompleteUploads:
			if _, err := s.component.CompleteTusUpload(event.Context, TusCompleteCommand{
				TusUploadID: event.Upload.ID,
				ObjectKey:   event.Upload.MetaData["objectKey"],
			}); err != nil {
				elog.Error("complete tus upload metadata failed", elog.String("upload_id", event.Upload.ID), elog.FieldErr(err))
			}
		case <-ticker.C:
		}
	}
}

func (s *TusServer) handle(ctx *gin.Context) {
	if ctx.Request.Method != http.MethodOptions {
		if _, err := authFromTusRequest(ctx.Request.Context(), ctx.Request.Header, s.opts); err != nil {
			ctx.Status(http.StatusUnauthorized)
			return
		}
	}
	http.StripPrefix(s.component.config.Tus.Path, s.handler).ServeHTTP(ctx.Writer, ctx.Request)
}

func (s *TusServer) Close() error {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
	})
	return nil
}

func (s *TusServer) AbortUpload(ctx context.Context, tusUploadID string) error {
	if tusUploadID == "" {
		return fmt.Errorf("upload: tus upload id is required")
	}
	upload, err := s.store.GetUpload(ctx, tusUploadID)
	if err != nil {
		return err
	}
	return s.store.AsTerminatableUpload(upload).Terminate(ctx)
}

func (c *Component) maxTusSize() int64 {
	var max int64
	for _, profile := range c.config.Profiles {
		if profile.MaxSize > max {
			max = profile.MaxSize
		}
	}
	return max
}
