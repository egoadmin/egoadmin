package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	uploadcomponent "github.com/egoadmin/egoadmin/internal/component/upload"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UploadRepository struct {
	db platformmysql.MysqlInterface
}

var _ uploadcomponent.MetadataStore = (*UploadRepository)(nil)

func NewUploadRepository(db platformmysql.MysqlInterface) *UploadRepository {
	return &UploadRepository{db: db}
}

func (r *UploadRepository) CreateMultipart(ctx context.Context, cmd uploadcomponent.CreateMultipartCommand) (*uploadcomponent.CreateMultipartResult, error) {
	var result uploadcomponent.CreateMultipartResult
	expiresAt := cmd.ExpiresAt
	err := r.db.Transaction(ctx, func(ctx context.Context) error {
		fileObject := fileObjectFromCommand(cmd)
		if err := r.db.WithTx(ctx).Create(fileObject).Error; err != nil {
			return err
		}
		reference := &fileReferenceModel{
			FileID:      fileObject.ID,
			OwnerUserID: cmd.OwnerUserID,
			Profile:     cmd.Profile,
			Status:      uploadcomponent.ReferenceStatusTemporary,
			ExpiresAt:   expiresAt,
		}
		if err := r.db.WithTx(ctx).Create(reference).Error; err != nil {
			return err
		}
		session := &uploadSessionModel{
			UploadType:  uploadcomponent.SessionTypeMultipart,
			FileID:      fileObject.ID,
			ReferenceID: reference.ID,
			ObjectKey:   cmd.ObjectKey,
			Status:      uploadcomponent.SessionStatusUploading,
			ExpiresAt:   expiresAt,
		}
		if err := r.db.WithTx(ctx).Create(session).Error; err != nil {
			return err
		}
		result = uploadcomponent.CreateMultipartResult{
			FileID:      fileObject.ID,
			ReferenceID: reference.ID,
			SessionID:   session.ID,
			ExpiresAt:   cmd.ExpiresAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *UploadRepository) CreateTus(ctx context.Context, cmd uploadcomponent.CreateTusCommand) (*uploadcomponent.CreateTusResult, error) {
	var result uploadcomponent.CreateTusResult
	err := r.db.Transaction(ctx, func(ctx context.Context) error {
		fileObject := fileObjectFromTusCommand(cmd)
		if err := r.db.WithTx(ctx).Create(fileObject).Error; err != nil {
			return err
		}
		reference := &fileReferenceModel{
			FileID:      fileObject.ID,
			OwnerUserID: cmd.OwnerUserID,
			Profile:     cmd.Profile,
			Status:      uploadcomponent.ReferenceStatusTemporary,
			ExpiresAt:   cmd.ExpiresAt,
		}
		if err := r.db.WithTx(ctx).Create(reference).Error; err != nil {
			return err
		}
		session := &uploadSessionModel{
			UploadType:  uploadcomponent.SessionTypeTus,
			TusUploadID: cmd.TusUploadID,
			FileID:      fileObject.ID,
			ReferenceID: reference.ID,
			ObjectKey:   cmd.ObjectKey,
			TusInfoKey:  cmd.TusInfoKey,
			TusPartKey:  cmd.TusPartKey,
			Status:      uploadcomponent.SessionStatusUploading,
			ExpiresAt:   cmd.ExpiresAt,
		}
		if err := r.db.WithTx(ctx).Create(session).Error; err != nil {
			return err
		}
		result = uploadcomponent.CreateTusResult{
			FileID:      fileObject.ID,
			ReferenceID: reference.ID,
			SessionID:   session.ID,
			ObjectKey:   fileObject.ObjectKey,
			TusUploadID: session.TusUploadID,
			TusInfoKey:  session.TusInfoKey,
			TusPartKey:  session.TusPartKey,
			ExpiresAt:   cmd.ExpiresAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *UploadRepository) MarkObjectAvailable(ctx context.Context, fileID uint64) error {
	now := time.Now()
	return r.db.WithTx(ctx).
		Model(&fileObjectModel{Model: xorm.Model{ID: fileID}}).
		Updates(map[string]any{
			"status":       uploadcomponent.ObjectStatusAvailable,
			"available_at": &now,
		}).
		Error
}

func (r *UploadRepository) MarkSessionFinished(ctx context.Context, sessionID uint64) error {
	now := time.Now()
	return r.db.WithTx(ctx).
		Model(&uploadSessionModel{Model: xorm.Model{ID: sessionID}}).
		Updates(map[string]any{
			"status":      uploadcomponent.SessionStatusFinished,
			"finished_at": &now,
		}).
		Error
}

func (r *UploadRepository) MarkUploadFailed(ctx context.Context, sessionID uint64) error {
	return r.db.Transaction(ctx, func(ctx context.Context) error {
		tx := r.db.WithTx(ctx)
		var session uploadSessionModel
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", sessionID, uploadcomponent.SessionStatusUploading).
			First(&session).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if err := tx.
			Model(&uploadSessionModel{Model: xorm.Model{ID: session.ID}}).
			Update("status", uploadcomponent.SessionStatusFailed).
			Error; err != nil {
			return err
		}
		if err := tx.
			Model(&fileReferenceModel{Model: xorm.Model{ID: session.ReferenceID}}).
			Where("status = ?", uploadcomponent.ReferenceStatusTemporary).
			Update("status", uploadcomponent.ReferenceStatusExpired).
			Error; err != nil {
			return err
		}
		return tx.
			Model(&fileObjectModel{Model: xorm.Model{ID: session.FileID}}).
			Where("status = ?", uploadcomponent.ObjectStatusUploading).
			Update("status", uploadcomponent.ObjectStatusDeleting).
			Error
	})
}

func (r *UploadRepository) MarkTusUploadFinished(ctx context.Context, cmd uploadcomponent.MarkTusUploadFinishedCommand) (*uploadcomponent.TusUploadDetail, error) {
	now := time.Now()
	var detail *uploadcomponent.TusUploadDetail
	err := r.db.Transaction(ctx, func(ctx context.Context) error {
		var session uploadSessionModel
		query := r.db.WithTx(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("upload_type = ?", uploadcomponent.SessionTypeTus)
		if cmd.ObjectKey != "" {
			query = query.Where("object_key = ?", cmd.ObjectKey)
		} else {
			query = query.Where("tus_upload_id = ?", cmd.TusUploadID)
		}
		if err := query.First(&session).Error; err != nil {
			return err
		}
		var file fileObjectModel
		if err := r.db.WithTx(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status IN ?", session.FileID, []string{uploadcomponent.ObjectStatusUploading, uploadcomponent.ObjectStatusAvailable}).
			First(&file).
			Error; err != nil {
			return err
		}
		if file.Status != uploadcomponent.ObjectStatusAvailable {
			if err := r.db.WithTx(ctx).
				Model(&fileObjectModel{Model: xorm.Model{ID: file.ID}}).
				Updates(map[string]any{
					"status":       uploadcomponent.ObjectStatusAvailable,
					"available_at": &now,
				}).
				Error; err != nil {
				return err
			}
			file.Status = uploadcomponent.ObjectStatusAvailable
			file.AvailableAt = &now
		}
		if session.Status != uploadcomponent.SessionStatusMetadataCleaned {
			if err := r.db.WithTx(ctx).
				Model(&uploadSessionModel{Model: xorm.Model{ID: session.ID}}).
				Where("status IN ?", []string{uploadcomponent.SessionStatusUploading, uploadcomponent.SessionStatusFinished, uploadcomponent.SessionStatusMetadataCleaning}).
				Updates(map[string]any{
					"status":        uploadcomponent.SessionStatusFinished,
					"finished_at":   &now,
					"tus_upload_id": cmd.TusUploadID,
				}).
				Error; err != nil {
				return err
			}
		}
		detail = &uploadcomponent.TusUploadDetail{
			FileID:      session.FileID,
			ReferenceID: session.ReferenceID,
			SessionID:   session.ID,
			ObjectKey:   session.ObjectKey,
			TusInfoKey:  session.TusInfoKey,
			TusPartKey:  session.TusPartKey,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (r *UploadRepository) MarkTusUploadCreated(ctx context.Context, cmd uploadcomponent.MarkTusUploadCreatedCommand) error {
	if cmd.TusUploadID == "" || cmd.ObjectKey == "" {
		return nil
	}
	return r.db.WithTx(ctx).
		Model(&uploadSessionModel{}).
		Where("upload_type = ? AND object_key = ? AND status = ?", uploadcomponent.SessionTypeTus, cmd.ObjectKey, uploadcomponent.SessionStatusUploading).
		Update("tus_upload_id", cmd.TusUploadID).
		Error
}

func (r *UploadRepository) FindExpiredTusUploads(ctx context.Context, now time.Time, limit int) ([]uploadcomponent.ExpiredTusUpload, error) {
	if limit <= 0 {
		limit = 100
	}
	sessions := make([]uploadSessionModel, 0, limit)
	err := r.db.WithTx(ctx).
		Where("upload_type = ? AND status = ? AND expires_at < ?", uploadcomponent.SessionTypeTus, uploadcomponent.SessionStatusUploading, now).
		Order("id ASC").
		Limit(limit).
		Find(&sessions).
		Error
	if err != nil {
		return nil, err
	}
	uploads := make([]uploadcomponent.ExpiredTusUpload, 0, len(sessions))
	for _, session := range sessions {
		uploads = append(uploads, uploadcomponent.ExpiredTusUpload{
			SessionID:   session.ID,
			FileID:      session.FileID,
			ReferenceID: session.ReferenceID,
			TusUploadID: session.TusUploadID,
			ObjectKey:   session.ObjectKey,
		})
	}
	return uploads, nil
}

func (r *UploadRepository) MarkTusUploadAborted(ctx context.Context, sessionID uint64) error {
	return r.db.Transaction(ctx, func(ctx context.Context) error {
		tx := r.db.WithTx(ctx)
		var session uploadSessionModel
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND upload_type = ? AND status = ?", sessionID, uploadcomponent.SessionTypeTus, uploadcomponent.SessionStatusUploading).
			First(&session).
			Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if err := tx.
			Model(&uploadSessionModel{Model: xorm.Model{ID: session.ID}}).
			Update("status", uploadcomponent.SessionStatusAborted).
			Error; err != nil {
			return err
		}
		if err := tx.
			Model(&fileReferenceModel{Model: xorm.Model{ID: session.ReferenceID}}).
			Where("status = ?", uploadcomponent.ReferenceStatusTemporary).
			Update("status", uploadcomponent.ReferenceStatusExpired).
			Error; err != nil {
			return err
		}
		return tx.
			Model(&fileObjectModel{Model: xorm.Model{ID: session.FileID}}).
			Where("status = ?", uploadcomponent.ObjectStatusUploading).
			Update("status", uploadcomponent.ObjectStatusDeleting).
			Error
	})
}

func (r *UploadRepository) FindReusableObject(ctx context.Context, cmd uploadcomponent.FindReusableObjectCommand) (*uploadcomponent.ReusableObject, error) {
	now := time.Now()
	var model fileObjectModel
	err := r.db.WithTx(ctx).
		Where("sha256 = ? AND size = ? AND status = ?", cmd.SHA256, cmd.Size, uploadcomponent.ObjectStatusAvailable).
		Where("EXISTS (?)",
			r.db.WithTx(ctx).
				Model(&fileReferenceModel{}).
				Select("1").
				Where("file_reference.file_id = file_object.id").
				Where("file_reference.owner_user_id = ?", cmd.OwnerUserID).
				Where("file_reference.profile = ?", cmd.Profile).
				Where(
					"file_reference.status = ? OR (file_reference.status = ? AND file_reference.expires_at >= ?)",
					uploadcomponent.ReferenceStatusBound,
					uploadcomponent.ReferenceStatusTemporary,
					now,
				),
		).
		Order("id ASC").
		First(&model).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &uploadcomponent.ReusableObject{
		FileID:    model.ID,
		ObjectKey: model.ObjectKey,
	}, nil
}

func (r *UploadRepository) CreateInstantReference(ctx context.Context, cmd uploadcomponent.CreateInstantReferenceCommand) (*uploadcomponent.CreateInstantReferenceResult, error) {
	var result uploadcomponent.CreateInstantReferenceResult
	err := r.db.Transaction(ctx, func(ctx context.Context) error {
		var file fileObjectModel
		if err := r.db.WithTx(ctx).
			Where("id = ? AND status = ?", cmd.FileID, uploadcomponent.ObjectStatusAvailable).
			First(&file).
			Error; err != nil {
			return err
		}
		reference := &fileReferenceModel{
			FileID:      file.ID,
			OwnerUserID: cmd.OwnerUserID,
			Profile:     cmd.Profile,
			Status:      uploadcomponent.ReferenceStatusTemporary,
			ExpiresAt:   cmd.ExpiresAt,
		}
		if err := r.db.WithTx(ctx).Create(reference).Error; err != nil {
			return err
		}
		result = uploadcomponent.CreateInstantReferenceResult{
			FileID:      file.ID,
			ReferenceID: reference.ID,
			ObjectKey:   file.ObjectKey,
			ExpiresAt:   cmd.ExpiresAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *UploadRepository) CommitReference(ctx context.Context, cmd uploadcomponent.CommitReferenceCommand) (*uploadcomponent.ReferenceDetail, error) {
	now := time.Now()
	var detail *uploadcomponent.ReferenceDetail
	err := r.db.Transaction(ctx, func(ctx context.Context) error {
		var reference fileReferenceModel
		err := r.db.WithTx(ctx).
			Where("id = ? AND owner_user_id = ?", cmd.ReferenceID, cmd.OwnerUserID).
			First(&reference).
			Error
		if err != nil {
			return err
		}
		if reference.Profile != cmd.Profile {
			return fmt.Errorf("upload: reference profile mismatch")
		}
		alreadyBound := reference.Status == uploadcomponent.ReferenceStatusBound &&
			reference.Service == cmd.Service &&
			reference.ResourceType == cmd.ResourceType &&
			reference.ResourceID == cmd.ResourceID &&
			reference.FieldName == cmd.FieldName
		if !alreadyBound && reference.Status != uploadcomponent.ReferenceStatusTemporary {
			return fmt.Errorf("upload reference status %s cannot be committed", reference.Status)
		}
		if !alreadyBound && time.Now().After(reference.ExpiresAt) {
			return fmt.Errorf("upload reference expired")
		}
		var file fileObjectModel
		if err := r.db.WithTx(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status = ?", reference.FileID, uploadcomponent.ObjectStatusAvailable).
			First(&file).
			Error; err != nil {
			return err
		}
		if !alreadyBound {
			if !cmd.DeferReleaseExisting {
				if err = r.releaseBoundReferencesForBinding(ctx, uploadcomponent.ReleasePreviousReferencesCommand{
					ReferenceID:  cmd.ReferenceID,
					Service:      cmd.Service,
					ResourceType: cmd.ResourceType,
					ResourceID:   cmd.ResourceID,
					FieldName:    cmd.FieldName,
				}, now); err != nil {
					return err
				}
			}
			if err = r.db.WithTx(ctx).
				Model(&fileReferenceModel{Model: xorm.Model{ID: reference.ID}}).
				Updates(map[string]any{
					"service":       cmd.Service,
					"resource_type": cmd.ResourceType,
					"resource_id":   cmd.ResourceID,
					"field_name":    cmd.FieldName,
					"status":        uploadcomponent.ReferenceStatusBound,
					"bound_at":      &now,
				}).
				Error; err != nil {
				return err
			}
			reference.Service = cmd.Service
			reference.ResourceType = cmd.ResourceType
			reference.ResourceID = cmd.ResourceID
			reference.FieldName = cmd.FieldName
			reference.Status = uploadcomponent.ReferenceStatusBound
			reference.BoundAt = &now
		}
		detail = referenceDetailFromModels(reference, file)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (r *UploadRepository) ReleasePreviousReferences(ctx context.Context, cmd uploadcomponent.ReleasePreviousReferencesCommand) error {
	return r.releaseBoundReferencesForBinding(ctx, cmd, time.Now())
}

func (r *UploadRepository) releaseBoundReferencesForBinding(ctx context.Context, cmd uploadcomponent.ReleasePreviousReferencesCommand, now time.Time) error {
	return r.db.WithTx(ctx).
		Model(&fileReferenceModel{}).
		Where("id <> ?", cmd.ReferenceID).
		Where("service = ? AND resource_type = ? AND resource_id = ? AND field_name = ?", cmd.Service, cmd.ResourceType, cmd.ResourceID, cmd.FieldName).
		Where("status = ?", uploadcomponent.ReferenceStatusBound).
		Updates(map[string]any{
			"status":      uploadcomponent.ReferenceStatusReleased,
			"released_at": &now,
		}).
		Error
}

func (r *UploadRepository) GetReference(ctx context.Context, referenceID uint64, ownerUserID uint64) (*uploadcomponent.ReferenceDetail, error) {
	var reference fileReferenceModel
	err := r.db.WithTx(ctx).
		Where("id = ? AND owner_user_id = ?", referenceID, ownerUserID).
		First(&reference).
		Error
	if err != nil {
		return nil, err
	}
	var file fileObjectModel
	if err = r.db.WithTx(ctx).
		Where("id = ?", reference.FileID).
		First(&file).
		Error; err != nil {
		return nil, err
	}
	return referenceDetailFromModels(reference, file), nil
}

func (r *UploadRepository) GetBoundReference(ctx context.Context, cmd uploadcomponent.GetBoundReferenceCommand) (*uploadcomponent.ReferenceDetail, error) {
	var reference fileReferenceModel
	err := r.db.WithTx(ctx).
		Where("service = ? AND resource_type = ? AND resource_id = ? AND field_name = ?", cmd.Service, cmd.ResourceType, cmd.ResourceID, cmd.FieldName).
		Where("profile = ? AND status = ?", cmd.Profile, uploadcomponent.ReferenceStatusBound).
		Order("bound_at DESC, id DESC").
		First(&reference).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, uploadcomponent.ErrReferenceNotFound
	}
	if err != nil {
		return nil, err
	}
	var file fileObjectModel
	if err = r.db.WithTx(ctx).
		Where("id = ?", reference.FileID).
		First(&file).
		Error; err != nil {
		return nil, err
	}
	return referenceDetailFromModels(reference, file), nil
}

func (r *UploadRepository) GetDownloadReference(ctx context.Context, referenceID uint64) (*uploadcomponent.DownloadObject, error) {
	return r.getDownloadReference(ctx, referenceID, 0, false)
}

func (r *UploadRepository) GetDownloadReferenceForOwner(ctx context.Context, referenceID uint64, ownerUserID uint64) (*uploadcomponent.DownloadObject, error) {
	return r.getDownloadReference(ctx, referenceID, ownerUserID, true)
}

func (r *UploadRepository) getDownloadReference(ctx context.Context, referenceID uint64, ownerUserID uint64, requireOwner bool) (*uploadcomponent.DownloadObject, error) {
	if referenceID == 0 {
		return nil, uploadcomponent.ErrReferenceNotFound
	}
	var reference fileReferenceModel
	query := r.db.WithTx(ctx).Where("id = ?", referenceID)
	if requireOwner {
		query = query.Where("owner_user_id = ?", ownerUserID)
	}
	err := query.First(&reference).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if requireOwner {
			var count int64
			if countErr := r.db.WithTx(ctx).
				Model(&fileReferenceModel{}).
				Where("id = ?", referenceID).
				Count(&count).
				Error; countErr != nil {
				return nil, countErr
			}
			if count > 0 {
				return nil, uploadcomponent.ErrReferenceForbidden
			}
		}
		return nil, uploadcomponent.ErrReferenceNotFound
	}
	if err != nil {
		return nil, err
	}
	var file fileObjectModel
	err = r.db.WithTx(ctx).
		Where("id = ?", reference.FileID).
		First(&file).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, uploadcomponent.ErrObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return downloadObjectFromModels(reference, file), nil
}

func referenceDetailFromModels(reference fileReferenceModel, file fileObjectModel) *uploadcomponent.ReferenceDetail {
	return &uploadcomponent.ReferenceDetail{
		ReferenceID: reference.ID,
		FileID:      reference.FileID,
		OwnerUserID: reference.OwnerUserID,
		Profile:     reference.Profile,
		Status:      reference.Status,
		ObjectKey:   file.ObjectKey,
		ExpiresAt:   reference.ExpiresAt,
	}
}

func downloadObjectFromModels(reference fileReferenceModel, file fileObjectModel) *uploadcomponent.DownloadObject {
	return &uploadcomponent.DownloadObject{
		ReferenceID:     reference.ID,
		FileID:          reference.FileID,
		OwnerUserID:     reference.OwnerUserID,
		Bucket:          file.Bucket,
		ObjectKey:       file.ObjectKey,
		OriginalName:    file.OriginalName,
		ContentType:     file.ContentType,
		Size:            file.Size,
		Profile:         reference.Profile,
		ReferenceStatus: reference.Status,
		ObjectStatus:    file.Status,
		ExpiresAt:       reference.ExpiresAt,
		AvailableAt:     file.AvailableAt,
	}
}

func (r *UploadRepository) ReleaseReference(ctx context.Context, cmd uploadcomponent.ReleaseReferenceCommand) error {
	now := time.Now()
	return r.db.WithTx(ctx).
		Model(&fileReferenceModel{}).
		Where("id = ? AND owner_user_id = ? AND status <> ?", cmd.ReferenceID, cmd.OwnerUserID, uploadcomponent.ReferenceStatusReleased).
		Updates(map[string]any{
			"status":      uploadcomponent.ReferenceStatusReleased,
			"released_at": &now,
		}).
		Error
}

func (r *UploadRepository) ExpireTemporaryReferences(ctx context.Context, now time.Time, limit int) ([]uploadcomponent.ExpiredReference, error) {
	if limit <= 0 {
		limit = 100
	}
	references := make([]fileReferenceModel, 0, limit)
	if err := r.db.WithTx(ctx).
		Where("status = ? AND expires_at < ?", uploadcomponent.ReferenceStatusTemporary, now).
		Order("id ASC").
		Limit(limit).
		Find(&references).
		Error; err != nil {
		return nil, err
	}
	if len(references) == 0 {
		return []uploadcomponent.ExpiredReference{}, nil
	}
	ids := make([]uint64, 0, len(references))
	expired := make([]uploadcomponent.ExpiredReference, 0, len(references))
	for _, reference := range references {
		ids = append(ids, reference.ID)
		expired = append(expired, uploadcomponent.ExpiredReference{
			ReferenceID: reference.ID,
			FileID:      reference.FileID,
		})
	}
	if err := r.db.WithTx(ctx).
		Model(&fileReferenceModel{}).
		Where("id IN ?", ids).
		Update("status", uploadcomponent.ReferenceStatusExpired).
		Error; err != nil {
		return nil, err
	}
	return expired, nil
}

func (r *UploadRepository) FindUnreferencedObjects(ctx context.Context, limit int) ([]uploadcomponent.UnreferencedObject, error) {
	if limit <= 0 {
		limit = 100
	}
	now := time.Now()
	models := make([]fileObjectModel, 0, limit)
	err := r.db.WithTx(ctx).
		Where("status IN ?", []string{uploadcomponent.ObjectStatusUploading, uploadcomponent.ObjectStatusAvailable, uploadcomponent.ObjectStatusDeleting}).
		Where("NOT EXISTS (?)",
			r.db.WithTx(ctx).
				Model(&fileReferenceModel{}).
				Select("1").
				Where("file_reference.file_id = file_object.id").
				Where(
					"file_reference.status = ? OR (file_reference.status = ? AND file_reference.expires_at >= ?)",
					uploadcomponent.ReferenceStatusBound,
					uploadcomponent.ReferenceStatusTemporary,
					now,
				),
		).
		Order("id ASC").
		Limit(limit).
		Find(&models).
		Error
	if err != nil {
		return nil, err
	}
	objects := make([]uploadcomponent.UnreferencedObject, 0, len(models))
	for _, model := range models {
		objects = append(objects, uploadcomponent.UnreferencedObject{
			FileID:    model.ID,
			ObjectKey: model.ObjectKey,
		})
	}
	return objects, nil
}

func (r *UploadRepository) MarkObjectDeleting(ctx context.Context, fileID uint64) (bool, error) {
	marked := false
	err := r.db.Transaction(ctx, func(ctx context.Context) error {
		tx := r.db.WithTx(ctx)
		var file fileObjectModel
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status IN ?", fileID, []string{uploadcomponent.ObjectStatusUploading, uploadcomponent.ObjectStatusAvailable, uploadcomponent.ObjectStatusDeleting}).
			First(&file).
			Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}

		var activeCount int64
		if err = tx.
			Model(&fileReferenceModel{}).
			Where("file_id = ?", fileID).
			Where("status = ? OR (status = ? AND expires_at >= ?)",
				uploadcomponent.ReferenceStatusBound,
				uploadcomponent.ReferenceStatusTemporary,
				time.Now(),
			).
			Count(&activeCount).
			Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return nil
		}
		if file.Status == uploadcomponent.ObjectStatusDeleting {
			marked = true
			return nil
		}

		result := tx.
			Model(&fileObjectModel{Model: xorm.Model{ID: fileID}}).
			Where("status IN ?", []string{uploadcomponent.ObjectStatusUploading, uploadcomponent.ObjectStatusAvailable}).
			Update("status", uploadcomponent.ObjectStatusDeleting)
		if result.Error != nil {
			return result.Error
		}
		marked = result.RowsAffected > 0
		return nil
	})
	return marked, err
}

func (r *UploadRepository) MarkObjectDeleted(ctx context.Context, fileID uint64) error {
	now := time.Now()
	return r.db.WithTx(ctx).
		Model(&fileObjectModel{Model: xorm.Model{ID: fileID}}).
		Where("status = ?", uploadcomponent.ObjectStatusDeleting).
		Updates(map[string]any{
			"status":     uploadcomponent.ObjectStatusDeleted,
			"deleted_at": gorm.DeletedAt{Time: now, Valid: true},
		}).
		Error
}

func (r *UploadRepository) FindTusMetadataForCleanup(ctx context.Context, limit int) ([]uploadcomponent.TusMetadataObject, error) {
	if limit <= 0 {
		limit = 100
	}
	sessions := make([]uploadSessionModel, 0, limit)
	err := r.db.WithTx(ctx).
		Model(&uploadSessionModel{}).
		Joins("JOIN file_object ON file_object.id = upload_session.file_id").
		Where("upload_session.upload_type = ?", uploadcomponent.SessionTypeTus).
		Where("upload_session.status IN ?", []string{uploadcomponent.SessionStatusFinished, uploadcomponent.SessionStatusMetadataCleaning}).
		Where("upload_session.metadata_cleaned_at IS NULL").
		Where("file_object.status = ?", uploadcomponent.ObjectStatusAvailable).
		Order("upload_session.id ASC").
		Limit(limit).
		Find(&sessions).
		Error
	if err != nil {
		return nil, err
	}
	objects := make([]uploadcomponent.TusMetadataObject, 0, len(sessions))
	for _, session := range sessions {
		objects = append(objects, uploadcomponent.TusMetadataObject{
			SessionID:  session.ID,
			FileID:     session.FileID,
			TusInfoKey: session.TusInfoKey,
			TusPartKey: session.TusPartKey,
		})
	}
	return objects, nil
}

func (r *UploadRepository) MarkTusMetadataCleaning(ctx context.Context, sessionID uint64) (bool, error) {
	marked := false
	err := r.db.Transaction(ctx, func(ctx context.Context) error {
		var session uploadSessionModel
		err := r.db.WithTx(ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND upload_type = ? AND status IN ?", sessionID, uploadcomponent.SessionTypeTus, []string{uploadcomponent.SessionStatusFinished, uploadcomponent.SessionStatusMetadataCleaning}).
			First(&session).
			Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var file fileObjectModel
		err = r.db.WithTx(ctx).
			Where("id = ? AND status = ?", session.FileID, uploadcomponent.ObjectStatusAvailable).
			First(&file).
			Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if session.Status == uploadcomponent.SessionStatusMetadataCleaning {
			marked = true
			return nil
		}
		result := r.db.WithTx(ctx).
			Model(&uploadSessionModel{Model: xorm.Model{ID: sessionID}}).
			Where("status = ?", uploadcomponent.SessionStatusFinished).
			Update("status", uploadcomponent.SessionStatusMetadataCleaning)
		if result.Error != nil {
			return result.Error
		}
		marked = result.RowsAffected > 0
		return nil
	})
	return marked, err
}

func (r *UploadRepository) MarkTusMetadataCleaned(ctx context.Context, sessionID uint64) error {
	now := time.Now()
	return r.db.WithTx(ctx).
		Model(&uploadSessionModel{Model: xorm.Model{ID: sessionID}}).
		Where("status = ?", uploadcomponent.SessionStatusMetadataCleaning).
		Updates(map[string]any{
			"status":              uploadcomponent.SessionStatusMetadataCleaned,
			"metadata_cleaned_at": &now,
		}).
		Error
}
