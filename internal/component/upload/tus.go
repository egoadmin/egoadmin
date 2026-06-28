package upload

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"
)

type TusCreateCommand struct {
	Profile      string
	OwnerUserID  uint64
	OriginalName string
	ContentType  string
	SHA256       string
	Size         int64
}

type TusCompleteCommand struct {
	TusUploadID string
	ObjectKey   string
}

func (c *Component) CreateTusUpload(ctx context.Context, cmd TusCreateCommand) (*CreateTusResult, error) {
	profileName, profile, err := c.config.RequireProfile(cmd.Profile)
	if err != nil {
		return nil, err
	}
	if err = validateTusUpload(profile, cmd); err != nil {
		return nil, err
	}
	fileID, err := c.flake.Get()
	if err != nil {
		return nil, fmt.Errorf("upload: generate tus file id: %w", err)
	}
	objectKey, err := c.objectKey(profileName, fileID, path.Ext(cmd.OriginalName))
	if err != nil {
		return nil, err
	}
	tusUploadID := c.tusUploadID(objectKey)
	expiresAt := time.Now().Add(profile.TTL)
	return c.store.CreateTus(ctx, CreateTusCommand{
		FileID:       fileID,
		Bucket:       c.bucket,
		ObjectKey:    objectKey,
		TusUploadID:  tusUploadID,
		TusInfoKey:   c.tusMetadataKey(tusUploadID, ".info"),
		TusPartKey:   c.tusMetadataKey(tusUploadID, ".part"),
		OriginalName: cmd.OriginalName,
		ContentType:  cmd.ContentType,
		SHA256:       cmd.SHA256,
		Size:         cmd.Size,
		CreatedBy:    cmd.OwnerUserID,
		OwnerUserID:  cmd.OwnerUserID,
		Profile:      profileName,
		ExpiresAt:    expiresAt,
	})
}

func (c *Component) CompleteTusUpload(ctx context.Context, cmd TusCompleteCommand) (*TusUploadDetail, error) {
	if cmd.TusUploadID == "" {
		return nil, fmt.Errorf("upload: tus upload id is required")
	}
	return c.store.MarkTusUploadFinished(ctx, MarkTusUploadFinishedCommand{
		TusUploadID: cmd.TusUploadID,
		ObjectKey:   cmd.ObjectKey,
	})
}

func (c *Component) tusMetadataKey(tusUploadID, suffix string) string {
	prefix := strings.Trim(c.config.Tus.MetadataPrefix, "/")
	if prefix == "" {
		prefix = "tus-meta"
	}
	trimmed := strings.TrimLeft(tusUploadID, "/")
	return path.Join(prefix, trimmed) + suffix
}

func (c *Component) tusUploadID(objectKey string) string {
	objectPrefix := strings.Trim(c.config.Tus.ObjectPrefix, "/")
	objectKey = strings.TrimLeft(objectKey, "/")
	if objectPrefix == "" {
		return objectKey
	}
	prefix := objectPrefix + "/"
	return strings.TrimPrefix(objectKey, prefix)
}

func (c *Component) metadataKeyAllowed(key string) bool {
	if key == "" {
		return false
	}
	prefix := strings.Trim(c.config.Tus.MetadataPrefix, "/")
	if prefix == "" {
		prefix = "tus-meta"
	}
	prefix += "/"
	key = strings.TrimLeft(key, "/")
	return strings.HasPrefix(key, prefix)
}

func validateTusUpload(profile ProfileConfig, cmd TusCreateCommand) error {
	return validateFileAttributes(profile, cmd.OriginalName, cmd.ContentType, cmd.Size)
}
