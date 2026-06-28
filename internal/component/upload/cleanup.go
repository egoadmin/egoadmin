package upload

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type CleanupResult struct {
	ExpiredReferences  int
	DeletedObjects     int
	CleanedTusMetadata int
	AbortedTusUploads  int
	DeletedTusTemp     int
}

func (c *Component) CleanupExpired(ctx context.Context, now time.Time, limit int) (*CleanupResult, error) {
	expired, err := c.store.ExpireTemporaryReferences(ctx, now, limit)
	if err != nil {
		return nil, err
	}
	objects, err := c.store.FindUnreferencedObjects(ctx, limit)
	if err != nil {
		return nil, err
	}
	result := &CleanupResult{ExpiredReferences: len(expired)}
	for _, object := range objects {
		marked, err := c.store.MarkObjectDeleting(ctx, object.FileID)
		if err != nil {
			return result, err
		}
		if !marked {
			continue
		}
		if err = c.object.Delete(ctx, object.ObjectKey); err != nil && !errors.Is(err, ErrObjectNotFound) {
			return result, err
		}
		if err = c.store.MarkObjectDeleted(ctx, object.FileID); err != nil {
			return result, err
		}
		result.DeletedObjects++
	}
	metadataObjects, err := c.store.FindTusMetadataForCleanup(ctx, limit)
	if err != nil {
		return result, err
	}
	for _, metadata := range metadataObjects {
		marked, err := c.store.MarkTusMetadataCleaning(ctx, metadata.SessionID)
		if err != nil {
			return result, err
		}
		if !marked {
			continue
		}
		if err = c.deleteTusMetadata(ctx, metadata); err != nil {
			return result, err
		}
		if err = c.store.MarkTusMetadataCleaned(ctx, metadata.SessionID); err != nil {
			return result, err
		}
		result.CleanedTusMetadata++
	}
	expiredTusUploads, err := c.store.FindExpiredTusUploads(ctx, now, limit)
	if err != nil {
		return result, err
	}
	for _, upload := range expiredTusUploads {
		if isCompleteTusUploadID(upload.TusUploadID) && c.tus != nil {
			if err = c.tus.AbortUpload(ctx, upload.TusUploadID); err != nil {
				return result, err
			}
		}
		if err = c.store.MarkTusUploadAborted(ctx, upload.SessionID); err != nil {
			return result, err
		}
		result.AbortedTusUploads++
	}
	localTemp, err := c.CleanupTusLocalTemp(ctx, now)
	if err != nil {
		return result, err
	}
	result.DeletedTusTemp = localTemp.DeletedFiles
	return result, nil
}

func (c *Component) deleteTusMetadata(ctx context.Context, metadata TusMetadataObject) error {
	for _, key := range []string{metadata.TusInfoKey, metadata.TusPartKey} {
		if key == "" {
			continue
		}
		if !c.metadataKeyAllowed(key) {
			return fmt.Errorf("upload: tus metadata key %q is outside metadata prefix", key)
		}
		if err := c.object.Delete(ctx, key); err != nil && !errors.Is(err, ErrObjectNotFound) {
			return err
		}
	}
	return nil
}

func isCompleteTusUploadID(uploadID string) bool {
	return strings.Contains(uploadID, "+")
}
