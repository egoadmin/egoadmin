package mysql

import (
	"context"
	"testing"
	"time"

	uploadcomponent "github.com/egoadmin/egoadmin/internal/component/upload"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"gorm.io/gorm"
)

func TestUploadRepository_CreateMultipartLifecycle(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 1000})
	repo := NewUploadRepository(testMysql{db: db})

	result, err := repo.CreateMultipart(ctx, uploadcomponent.CreateMultipartCommand{
		FileID:       1001,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-1001.png",
		OriginalName: "avatar.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateMultipart() error = %v", err)
	}
	if result.FileID == 0 || result.ReferenceID == 0 || result.SessionID == 0 {
		t.Fatalf("result = %#v, want generated ids", result)
	}

	var file fileObjectModel
	if err = db.First(&file, result.FileID).Error; err != nil {
		t.Fatalf("get file object: %v", err)
	}
	if file.Status != uploadcomponent.ObjectStatusUploading || file.ObjectKey == "" {
		t.Fatalf("file = %#v, want uploading object", file)
	}

	var reference fileReferenceModel
	if err = db.First(&reference, result.ReferenceID).Error; err != nil {
		t.Fatalf("get file reference: %v", err)
	}
	if reference.Status != uploadcomponent.ReferenceStatusTemporary || reference.FileID != result.FileID {
		t.Fatalf("reference = %#v, want temporary reference", reference)
	}

	if err = repo.MarkObjectAvailable(ctx, result.FileID); err != nil {
		t.Fatalf("MarkObjectAvailable() error = %v", err)
	}
	if err = repo.MarkSessionFinished(ctx, result.SessionID); err != nil {
		t.Fatalf("MarkSessionFinished() error = %v", err)
	}

	if err = db.First(&file, result.FileID).Error; err != nil {
		t.Fatalf("reload file object: %v", err)
	}
	if file.Status != uploadcomponent.ObjectStatusAvailable || file.AvailableAt == nil {
		t.Fatalf("file = %#v, want available object", file)
	}

	var session uploadSessionModel
	if err = db.First(&session, result.SessionID).Error; err != nil {
		t.Fatalf("get upload session: %v", err)
	}
	if session.Status != uploadcomponent.SessionStatusFinished || session.FinishedAt == nil {
		t.Fatalf("session = %#v, want finished session", session)
	}
}

func TestUploadRepository_MarkUploadFailedMakesObjectCleanable(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 1100})
	repo := NewUploadRepository(testMysql{db: db})

	result, err := repo.CreateMultipart(ctx, uploadcomponent.CreateMultipartCommand{
		FileID:       1101,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-1101.png",
		OriginalName: "avatar.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateMultipart() error = %v", err)
	}
	if err = repo.MarkUploadFailed(ctx, result.SessionID); err != nil {
		t.Fatalf("MarkUploadFailed() error = %v", err)
	}

	var session uploadSessionModel
	if err = db.First(&session, result.SessionID).Error; err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != uploadcomponent.SessionStatusFailed {
		t.Fatalf("session status = %q, want failed", session.Status)
	}
	var reference fileReferenceModel
	if err = db.First(&reference, result.ReferenceID).Error; err != nil {
		t.Fatalf("get reference: %v", err)
	}
	if reference.Status != uploadcomponent.ReferenceStatusExpired {
		t.Fatalf("reference status = %q, want expired", reference.Status)
	}
	var file fileObjectModel
	if err = db.First(&file, result.FileID).Error; err != nil {
		t.Fatalf("get file: %v", err)
	}
	if file.Status != uploadcomponent.ObjectStatusDeleting {
		t.Fatalf("file status = %q, want deleting", file.Status)
	}
	objects, err := repo.FindUnreferencedObjects(ctx, 10)
	if err != nil {
		t.Fatalf("FindUnreferencedObjects() error = %v", err)
	}
	if len(objects) != 1 || objects[0].FileID != result.FileID {
		t.Fatalf("objects = %#v, want failed file object", objects)
	}
}

func TestUploadRepository_TusLifecycleAndMetadataCleanup(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 1500})
	repo := NewUploadRepository(testMysql{db: db})

	result, err := repo.CreateTus(ctx, uploadcomponent.CreateTusCommand{
		FileID:       1501,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-1501.png",
		TusUploadID:  "files/avatar/2026/06/26/file-test-1501.png",
		TusInfoKey:   "tus-meta/avatar/2026/06/26/file-test-1501.png.info",
		TusPartKey:   "tus-meta/avatar/2026/06/26/file-test-1501.png.part",
		OriginalName: "avatar.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateTus() error = %v", err)
	}
	if result.FileID == 0 || result.ReferenceID == 0 || result.SessionID == 0 {
		t.Fatalf("result = %#v, want generated ids", result)
	}

	if err = repo.MarkTusUploadCreated(ctx, uploadcomponent.MarkTusUploadCreatedCommand{
		TusUploadID: result.TusUploadID + "+multipart",
		ObjectKey:   result.ObjectKey,
	}); err != nil {
		t.Fatalf("MarkTusUploadCreated() error = %v", err)
	}
	var createdSession uploadSessionModel
	if err = db.First(&createdSession, result.SessionID).Error; err != nil {
		t.Fatalf("get created session: %v", err)
	}
	if createdSession.TusUploadID != result.TusUploadID+"+multipart" {
		t.Fatalf("created session tus upload id = %q, want complete id", createdSession.TusUploadID)
	}

	detail, err := repo.MarkTusUploadFinished(ctx, uploadcomponent.MarkTusUploadFinishedCommand{
		TusUploadID: result.TusUploadID + "+multipart",
		ObjectKey:   result.ObjectKey,
	})
	if err != nil {
		t.Fatalf("MarkTusUploadFinished() error = %v", err)
	}
	if detail == nil || detail.FileID != result.FileID || detail.TusInfoKey != result.TusInfoKey {
		t.Fatalf("detail = %#v, want tus upload detail", detail)
	}

	var file fileObjectModel
	if err = db.First(&file, result.FileID).Error; err != nil {
		t.Fatalf("get file object: %v", err)
	}
	if file.Status != uploadcomponent.ObjectStatusAvailable || file.AvailableAt == nil {
		t.Fatalf("file = %#v, want available object", file)
	}

	metadataObjects, err := repo.FindTusMetadataForCleanup(ctx, 10)
	if err != nil {
		t.Fatalf("FindTusMetadataForCleanup() error = %v", err)
	}
	if len(metadataObjects) != 1 || metadataObjects[0].SessionID != result.SessionID {
		t.Fatalf("metadataObjects = %#v, want session %d", metadataObjects, result.SessionID)
	}

	marked, err := repo.MarkTusMetadataCleaning(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("MarkTusMetadataCleaning() error = %v", err)
	}
	if !marked {
		t.Fatalf("MarkTusMetadataCleaning() = false, want true")
	}
	if err = repo.MarkTusMetadataCleaned(ctx, result.SessionID); err != nil {
		t.Fatalf("MarkTusMetadataCleaned() error = %v", err)
	}

	var session uploadSessionModel
	if err = db.First(&session, result.SessionID).Error; err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != uploadcomponent.SessionStatusMetadataCleaned || session.MetadataCleanedAt == nil {
		t.Fatalf("session = %#v, want metadata cleaned", session)
	}
	if session.TusUploadID != result.TusUploadID+"+multipart" {
		t.Fatalf("session tus upload id = %q, want completed tus id", session.TusUploadID)
	}

	metadataObjects, err = repo.FindTusMetadataForCleanup(ctx, 10)
	if err != nil {
		t.Fatalf("FindTusMetadataForCleanup(after) error = %v", err)
	}
	if len(metadataObjects) != 0 {
		t.Fatalf("metadataObjects after cleanup = %#v, want none", metadataObjects)
	}
}

func TestUploadRepository_ExpiredTusUploadAbortLifecycle(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 1600})
	repo := NewUploadRepository(testMysql{db: db})

	result, err := repo.CreateTus(ctx, uploadcomponent.CreateTusCommand{
		FileID:       1601,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-1601.png",
		TusUploadID:  "avatar/2026/06/26/file-test-1601.png+multipart",
		TusInfoKey:   "tus-meta/avatar/2026/06/26/file-test-1601.png.info",
		TusPartKey:   "tus-meta/avatar/2026/06/26/file-test-1601.png.part",
		OriginalName: "avatar.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateTus() error = %v", err)
	}

	uploads, err := repo.FindExpiredTusUploads(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("FindExpiredTusUploads() error = %v", err)
	}
	if len(uploads) != 1 || uploads[0].SessionID != result.SessionID || uploads[0].TusUploadID != result.TusUploadID {
		t.Fatalf("uploads = %#v, want expired upload session %d", uploads, result.SessionID)
	}

	if err = repo.MarkTusUploadAborted(ctx, result.SessionID); err != nil {
		t.Fatalf("MarkTusUploadAborted() error = %v", err)
	}

	var session uploadSessionModel
	if err = db.First(&session, result.SessionID).Error; err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != uploadcomponent.SessionStatusAborted {
		t.Fatalf("session status = %q, want aborted", session.Status)
	}
	var reference fileReferenceModel
	if err = db.First(&reference, result.ReferenceID).Error; err != nil {
		t.Fatalf("get reference: %v", err)
	}
	if reference.Status != uploadcomponent.ReferenceStatusExpired {
		t.Fatalf("reference status = %q, want expired", reference.Status)
	}
	var file fileObjectModel
	if err = db.First(&file, result.FileID).Error; err != nil {
		t.Fatalf("get file object: %v", err)
	}
	if file.Status != uploadcomponent.ObjectStatusDeleting {
		t.Fatalf("file status = %q, want deleting", file.Status)
	}
}

func TestUploadRepository_ReferenceLifecycle(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 2000})
	repo := NewUploadRepository(testMysql{db: db})

	result, err := repo.CreateMultipart(ctx, uploadcomponent.CreateMultipartCommand{
		FileID:       2001,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-2001.png",
		OriginalName: "avatar.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateMultipart() error = %v", err)
	}
	if err = repo.MarkObjectAvailable(ctx, result.FileID); err != nil {
		t.Fatalf("MarkObjectAvailable() error = %v", err)
	}
	committed, err := repo.CommitReference(ctx, uploadcomponent.CommitReferenceCommand{
		ReferenceID:  result.ReferenceID,
		OwnerUserID:  20001,
		Profile:      "avatar",
		Service:      "user",
		ResourceType: "user",
		ResourceID:   20001,
		FieldName:    "avatar",
	})
	if err != nil {
		t.Fatalf("CommitReference() error = %v", err)
	}
	if committed == nil || committed.ObjectKey == "" || committed.Status != uploadcomponent.ReferenceStatusBound {
		t.Fatalf("committed = %#v, want bound reference detail", committed)
	}

	var reference fileReferenceModel
	if err = db.First(&reference, result.ReferenceID).Error; err != nil {
		t.Fatalf("get reference: %v", err)
	}
	if reference.Status != uploadcomponent.ReferenceStatusBound || reference.BoundAt == nil {
		t.Fatalf("reference = %#v, want bound", reference)
	}

	if err = repo.ReleaseReference(ctx, uploadcomponent.ReleaseReferenceCommand{
		ReferenceID: result.ReferenceID,
		OwnerUserID: 20001,
	}); err != nil {
		t.Fatalf("ReleaseReference() error = %v", err)
	}
	if err = db.First(&reference, result.ReferenceID).Error; err != nil {
		t.Fatalf("reload reference: %v", err)
	}
	if reference.Status != uploadcomponent.ReferenceStatusReleased || reference.ReleasedAt == nil {
		t.Fatalf("reference = %#v, want released", reference)
	}
}

func TestUploadRepository_GetDownloadReference(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 2050})
	repo := NewUploadRepository(testMysql{db: db})

	result, err := repo.CreateMultipart(ctx, uploadcomponent.CreateMultipartCommand{
		FileID:       2051,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-2051.png",
		OriginalName: "avatar.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateMultipart() error = %v", err)
	}
	if err = repo.MarkObjectAvailable(ctx, result.FileID); err != nil {
		t.Fatalf("MarkObjectAvailable() error = %v", err)
	}
	object, err := repo.GetDownloadReference(ctx, result.ReferenceID)
	if err != nil {
		t.Fatalf("GetDownloadReference() error = %v", err)
	}
	if object.ReferenceID != result.ReferenceID ||
		object.FileID != result.FileID ||
		object.OwnerUserID != 20001 ||
		object.Bucket != "egoadmin" ||
		object.ObjectKey != "files/avatar/2026/06/26/file-test-2051.png" ||
		object.OriginalName != "avatar.png" ||
		object.ContentType != "image/png" ||
		object.Size != 12 ||
		object.Profile != "avatar" ||
		object.ReferenceStatus != uploadcomponent.ReferenceStatusTemporary ||
		object.ObjectStatus != uploadcomponent.ObjectStatusAvailable ||
		object.AvailableAt == nil {
		t.Fatalf("download object = %#v, want complete download metadata", object)
	}
	ownerObject, err := repo.GetDownloadReferenceForOwner(ctx, result.ReferenceID, 20001)
	if err != nil {
		t.Fatalf("GetDownloadReferenceForOwner() error = %v", err)
	}
	if ownerObject.ReferenceID != result.ReferenceID {
		t.Fatalf("owner object = %#v, want reference %d", ownerObject, result.ReferenceID)
	}
	_, err = repo.GetDownloadReferenceForOwner(ctx, result.ReferenceID, 20002)
	if err != uploadcomponent.ErrReferenceForbidden {
		t.Fatalf("GetDownloadReferenceForOwner(other user) error = %v, want forbidden", err)
	}
}

func TestUploadRepository_CommitReferenceReleasesPreviousBinding(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 2100})
	repo := NewUploadRepository(testMysql{db: db})

	oldUpload, err := repo.CreateMultipart(ctx, uploadcomponent.CreateMultipartCommand{
		FileID:       2101,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-2101.png",
		OriginalName: "old.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateMultipart(old) error = %v", err)
	}
	newUpload, err := repo.CreateMultipart(ctx, uploadcomponent.CreateMultipartCommand{
		FileID:       2104,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-2104.png",
		OriginalName: "new.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateMultipart(new) error = %v", err)
	}
	for _, fileID := range []uint64{oldUpload.FileID, newUpload.FileID} {
		if err = repo.MarkObjectAvailable(ctx, fileID); err != nil {
			t.Fatalf("MarkObjectAvailable(%d) error = %v", fileID, err)
		}
	}
	binding := uploadcomponent.CommitReferenceCommand{
		OwnerUserID:  20001,
		Profile:      "avatar",
		Service:      "user",
		ResourceType: "user",
		ResourceID:   20001,
		FieldName:    "avatar",
	}
	binding.ReferenceID = oldUpload.ReferenceID
	if _, err = repo.CommitReference(ctx, binding); err != nil {
		t.Fatalf("CommitReference(old) error = %v", err)
	}
	binding.ReferenceID = newUpload.ReferenceID
	if _, err = repo.CommitReference(ctx, binding); err != nil {
		t.Fatalf("CommitReference(new) error = %v", err)
	}

	var oldReference fileReferenceModel
	if err = db.First(&oldReference, oldUpload.ReferenceID).Error; err != nil {
		t.Fatalf("get old reference: %v", err)
	}
	if oldReference.Status != uploadcomponent.ReferenceStatusReleased || oldReference.ReleasedAt == nil {
		t.Fatalf("old reference = %#v, want released", oldReference)
	}
	var newReference fileReferenceModel
	if err = db.First(&newReference, newUpload.ReferenceID).Error; err != nil {
		t.Fatalf("get new reference: %v", err)
	}
	if newReference.Status != uploadcomponent.ReferenceStatusBound || newReference.ReleasedAt != nil {
		t.Fatalf("new reference = %#v, want bound and not released", newReference)
	}
}

func TestUploadRepository_CommitReferenceCanDeferPreviousRelease(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 2200})
	repo := NewUploadRepository(testMysql{db: db})

	oldUpload, err := repo.CreateMultipart(ctx, uploadcomponent.CreateMultipartCommand{
		FileID:       2201,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-2201.png",
		OriginalName: "old.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateMultipart(old) error = %v", err)
	}
	newUpload, err := repo.CreateMultipart(ctx, uploadcomponent.CreateMultipartCommand{
		FileID:       2204,
		Bucket:       "egoadmin",
		ObjectKey:    "files/avatar/2026/06/26/file-test-2204.png",
		OriginalName: "new.png",
		ContentType:  "image/png",
		Size:         12,
		CreatedBy:    20001,
		OwnerUserID:  20001,
		Profile:      "avatar",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateMultipart(new) error = %v", err)
	}
	for _, fileID := range []uint64{oldUpload.FileID, newUpload.FileID} {
		if err = repo.MarkObjectAvailable(ctx, fileID); err != nil {
			t.Fatalf("MarkObjectAvailable(%d) error = %v", fileID, err)
		}
	}
	binding := uploadcomponent.CommitReferenceCommand{
		OwnerUserID:  20001,
		Profile:      "avatar",
		Service:      "user",
		ResourceType: "user",
		ResourceID:   20001,
		FieldName:    "avatar",
	}
	binding.ReferenceID = oldUpload.ReferenceID
	if _, err = repo.CommitReference(ctx, binding); err != nil {
		t.Fatalf("CommitReference(old) error = %v", err)
	}
	binding.ReferenceID = newUpload.ReferenceID
	binding.DeferReleaseExisting = true
	if _, err = repo.CommitReference(ctx, binding); err != nil {
		t.Fatalf("CommitReference(new) error = %v", err)
	}

	var oldReference fileReferenceModel
	if err = db.First(&oldReference, oldUpload.ReferenceID).Error; err != nil {
		t.Fatalf("get old reference: %v", err)
	}
	if oldReference.Status != uploadcomponent.ReferenceStatusBound {
		t.Fatalf("old reference status = %q, want still bound before explicit release", oldReference.Status)
	}
	if err = repo.ReleasePreviousReferences(ctx, uploadcomponent.ReleasePreviousReferencesCommand{
		ReferenceID:  newUpload.ReferenceID,
		Service:      "user",
		ResourceType: "user",
		ResourceID:   20001,
		FieldName:    "avatar",
	}); err != nil {
		t.Fatalf("ReleasePreviousReferences() error = %v", err)
	}
	if err = db.First(&oldReference, oldUpload.ReferenceID).Error; err != nil {
		t.Fatalf("reload old reference: %v", err)
	}
	if oldReference.Status != uploadcomponent.ReferenceStatusReleased || oldReference.ReleasedAt == nil {
		t.Fatalf("old reference = %#v, want released after explicit release", oldReference)
	}
}

func TestUploadRepository_InstantAndExpire(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 3000})
	repo := NewUploadRepository(testMysql{db: db})

	file := &fileObjectModel{
		Bucket:    "egoadmin",
		ObjectKey: "files/document/2026/06/26/file-test-3001.pdf",
		SHA256:    "abc",
		Size:      12,
		Status:    uploadcomponent.ObjectStatusAvailable,
		CreatedBy: 20001,
	}
	if err := db.Create(file).Error; err != nil {
		t.Fatalf("create reusable file: %v", err)
	}
	notReusableWithoutProfileReference, err := repo.FindReusableObject(ctx, uploadcomponent.FindReusableObjectCommand{
		SHA256:      "abc",
		Size:        12,
		OwnerUserID: 20001,
		Profile:     "document",
	})
	if err != nil {
		t.Fatalf("FindReusableObject(owner without profile reference) error = %v", err)
	}
	if notReusableWithoutProfileReference != nil {
		t.Fatalf("reusable without profile reference = %#v, want nil", notReusableWithoutProfileReference)
	}
	notReusableForOtherUser, err := repo.FindReusableObject(ctx, uploadcomponent.FindReusableObjectCommand{
		SHA256:      "abc",
		Size:        12,
		OwnerUserID: 20002,
		Profile:     "document",
	})
	if err != nil {
		t.Fatalf("FindReusableObject(other user) error = %v", err)
	}
	if notReusableForOtherUser != nil {
		t.Fatalf("reusable for other user = %#v, want nil", notReusableForOtherUser)
	}
	if err := db.Create(&fileReferenceModel{
		FileID:      file.ID,
		OwnerUserID: 20002,
		Profile:     "avatar",
		Status:      uploadcomponent.ReferenceStatusTemporary,
		ExpiresAt:   time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("create other profile reference: %v", err)
	}
	notReusableForOtherProfile, err := repo.FindReusableObject(ctx, uploadcomponent.FindReusableObjectCommand{
		SHA256:      "abc",
		Size:        12,
		OwnerUserID: 20002,
		Profile:     "document",
	})
	if err != nil {
		t.Fatalf("FindReusableObject(other profile) error = %v", err)
	}
	if notReusableForOtherProfile != nil {
		t.Fatalf("reusable for other profile = %#v, want nil", notReusableForOtherProfile)
	}
	notReusableForOwnerOtherProfile, err := repo.FindReusableObject(ctx, uploadcomponent.FindReusableObjectCommand{
		SHA256:      "abc",
		Size:        12,
		OwnerUserID: 20001,
		Profile:     "avatar",
	})
	if err != nil {
		t.Fatalf("FindReusableObject(owner without avatar reference) error = %v", err)
	}
	if notReusableForOwnerOtherProfile != nil {
		t.Fatalf("reusable for owner other profile = %#v, want nil", notReusableForOwnerOtherProfile)
	}
	if err := db.Create(&fileReferenceModel{
		FileID:      file.ID,
		OwnerUserID: 20001,
		Profile:     "document",
		Status:      uploadcomponent.ReferenceStatusTemporary,
		ExpiresAt:   time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("create owner profile reference: %v", err)
	}
	reusableWithProfileReference, err := repo.FindReusableObject(ctx, uploadcomponent.FindReusableObjectCommand{
		SHA256:      "abc",
		Size:        12,
		OwnerUserID: 20001,
		Profile:     "document",
	})
	if err != nil {
		t.Fatalf("FindReusableObject(owner profile reference) error = %v", err)
	}
	if reusableWithProfileReference == nil || reusableWithProfileReference.FileID != file.ID {
		t.Fatalf("reusable with profile reference = %#v, want file %d", reusableWithProfileReference, file.ID)
	}

	instant, err := repo.CreateInstantReference(ctx, uploadcomponent.CreateInstantReferenceCommand{
		FileID:      file.ID,
		OwnerUserID: 20001,
		Profile:     "document",
		ExpiresAt:   time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateInstantReference() error = %v", err)
	}
	expired, err := repo.ExpireTemporaryReferences(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("ExpireTemporaryReferences() error = %v", err)
	}
	if len(expired) != 1 || expired[0].ReferenceID != instant.ReferenceID {
		t.Fatalf("expired = %#v, want reference %d", expired, instant.ReferenceID)
	}
}

func TestUploadRepository_UnreferencedObjectCleanup(t *testing.T) {
	ctx := context.Background()
	db := newUploadTestDB(t)
	platformmysql.NewID(&testFlake{next: 4000})
	repo := NewUploadRepository(testMysql{db: db})

	available := &fileObjectModel{
		Bucket:    "egoadmin",
		ObjectKey: "files/avatar/2026/06/26/file-test-4001.png",
		Status:    uploadcomponent.ObjectStatusAvailable,
		CreatedBy: 20001,
	}
	bound := &fileObjectModel{
		Bucket:    "egoadmin",
		ObjectKey: "files/avatar/2026/06/26/file-test-4002.png",
		Status:    uploadcomponent.ObjectStatusAvailable,
		CreatedBy: 20001,
	}
	temporary := &fileObjectModel{
		Bucket:    "egoadmin",
		ObjectKey: "files/avatar/2026/06/26/file-test-4004.png",
		Status:    uploadcomponent.ObjectStatusAvailable,
		CreatedBy: 20001,
	}
	expiredTemporary := &fileObjectModel{
		Bucket:    "egoadmin",
		ObjectKey: "files/avatar/2026/06/26/file-test-4005.png",
		Status:    uploadcomponent.ObjectStatusAvailable,
		CreatedBy: 20001,
	}
	retry := &fileObjectModel{
		Bucket:    "egoadmin",
		ObjectKey: "files/avatar/2026/06/26/file-test-4003.png",
		Status:    uploadcomponent.ObjectStatusDeleting,
		CreatedBy: 20001,
	}
	if err := db.Create(available).Error; err != nil {
		t.Fatalf("create available file: %v", err)
	}
	if err := db.Create(bound).Error; err != nil {
		t.Fatalf("create bound file: %v", err)
	}
	if err := db.Create(temporary).Error; err != nil {
		t.Fatalf("create temporary file: %v", err)
	}
	if err := db.Create(expiredTemporary).Error; err != nil {
		t.Fatalf("create expired temporary file: %v", err)
	}
	if err := db.Create(retry).Error; err != nil {
		t.Fatalf("create retry file: %v", err)
	}
	if err := db.Create(&fileReferenceModel{
		FileID:      bound.ID,
		OwnerUserID: 20001,
		Profile:     "avatar",
		Status:      uploadcomponent.ReferenceStatusBound,
		ExpiresAt:   time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("create bound reference: %v", err)
	}
	if err := db.Create(&fileReferenceModel{
		FileID:      temporary.ID,
		OwnerUserID: 20001,
		Profile:     "avatar",
		Status:      uploadcomponent.ReferenceStatusTemporary,
		ExpiresAt:   time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("create temporary reference: %v", err)
	}
	if err := db.Create(&fileReferenceModel{
		FileID:      expiredTemporary.ID,
		OwnerUserID: 20001,
		Profile:     "avatar",
		Status:      uploadcomponent.ReferenceStatusTemporary,
		ExpiresAt:   time.Now().Add(-time.Hour),
	}).Error; err != nil {
		t.Fatalf("create expired temporary reference: %v", err)
	}

	objects, err := repo.FindUnreferencedObjects(ctx, 10)
	if err != nil {
		t.Fatalf("FindUnreferencedObjects() error = %v", err)
	}
	got := map[uint64]bool{}
	for _, object := range objects {
		got[object.FileID] = true
	}
	if !got[available.ID] || !got[retry.ID] {
		t.Fatalf("objects = %#v, want available and deleting retry objects", objects)
	}
	if !got[expiredTemporary.ID] {
		t.Fatalf("objects = %#v, expired temporary object should be returned", objects)
	}
	if got[bound.ID] {
		t.Fatalf("objects = %#v, bound object should not be returned", objects)
	}
	if got[temporary.ID] {
		t.Fatalf("objects = %#v, temporary object should not be returned", objects)
	}

	marked, err := repo.MarkObjectDeleting(ctx, available.ID)
	if err != nil {
		t.Fatalf("MarkObjectDeleting(available) error = %v", err)
	}
	if !marked {
		t.Fatalf("MarkObjectDeleting(available) = false, want true")
	}
	marked, err = repo.MarkObjectDeleting(ctx, bound.ID)
	if err != nil {
		t.Fatalf("MarkObjectDeleting(bound) error = %v", err)
	}
	if marked {
		t.Fatalf("MarkObjectDeleting(bound) = true, want false")
	}
	marked, err = repo.MarkObjectDeleting(ctx, temporary.ID)
	if err != nil {
		t.Fatalf("MarkObjectDeleting(temporary) error = %v", err)
	}
	if marked {
		t.Fatalf("MarkObjectDeleting(temporary) = true, want false")
	}
	marked, err = repo.MarkObjectDeleting(ctx, expiredTemporary.ID)
	if err != nil {
		t.Fatalf("MarkObjectDeleting(expired temporary) error = %v", err)
	}
	if !marked {
		t.Fatalf("MarkObjectDeleting(expired temporary) = false, want true")
	}
	marked, err = repo.MarkObjectDeleting(ctx, retry.ID)
	if err != nil {
		t.Fatalf("MarkObjectDeleting(retry) error = %v", err)
	}
	if !marked {
		t.Fatalf("MarkObjectDeleting(retry) = false, want true")
	}

	if err = repo.MarkObjectDeleted(ctx, available.ID); err != nil {
		t.Fatalf("MarkObjectDeleted() error = %v", err)
	}
	var deleted fileObjectModel
	if err = db.Unscoped().First(&deleted, available.ID).Error; err != nil {
		t.Fatalf("get deleted object: %v", err)
	}
	if deleted.Status != uploadcomponent.ObjectStatusDeleted || !deleted.DeletedAt.Valid {
		t.Fatalf("deleted object = %#v, want deleted status and soft delete time", deleted)
	}
}

func newUploadTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t)
	if err := db.Exec(`CREATE TABLE file_object (
		id integer PRIMARY KEY,
		created_at datetime,
		updated_at datetime,
		deleted_at datetime,
		bucket text,
		object_key text,
		sha256 text,
		size integer,
		content_type text,
		original_name text,
		status text,
		created_by integer,
		available_at datetime
	)`).Error; err != nil {
		t.Fatalf("create file_object table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE file_reference (
		id integer PRIMARY KEY,
		created_at datetime,
		updated_at datetime,
		deleted_at datetime,
		file_id integer,
		owner_user_id integer,
		profile text,
		service text,
		resource_type text,
		resource_id integer,
		field_name text,
		status text,
		expires_at datetime,
		bound_at datetime,
		released_at datetime
	)`).Error; err != nil {
		t.Fatalf("create file_reference table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE upload_session (
		id integer PRIMARY KEY,
		created_at datetime,
		updated_at datetime,
		deleted_at datetime,
		upload_type text,
		tus_upload_id text,
		file_id integer,
		reference_id integer,
		object_key text,
		tus_info_key text,
		tus_part_key text,
		status text,
		finished_at datetime,
		metadata_cleaned_at datetime,
		expires_at datetime
	)`).Error; err != nil {
		t.Fatalf("create upload_session table: %v", err)
	}
	return db
}

type testFlake struct {
	next uint64
}

func (f *testFlake) Get() (uint64, error) {
	f.next++
	return f.next, nil
}
