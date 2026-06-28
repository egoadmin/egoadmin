package upload

import (
	"context"
	"io"
	"time"

	tuss3store "github.com/tus/tusd/v2/pkg/s3store"
)

type ObjectStore interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64, opts PutOptions) error
	Get(ctx context.Context, key string) (ObjectReader, error)
	Delete(ctx context.Context, key string) error
	Stat(ctx context.Context, key string) (ObjectInfo, error)
}

type TusS3API interface {
	tuss3store.S3API
}

type PutOptions struct {
	ContentType string
}

type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
}

type ObjectReader interface {
	io.ReadSeeker
	io.Closer
	Stat() (ObjectInfo, error)
}

type MetadataStore interface {
	CreateMultipart(ctx context.Context, cmd CreateMultipartCommand) (*CreateMultipartResult, error)
	CreateTus(ctx context.Context, cmd CreateTusCommand) (*CreateTusResult, error)
	CreateInstantReference(ctx context.Context, cmd CreateInstantReferenceCommand) (*CreateInstantReferenceResult, error)
	CommitReference(ctx context.Context, cmd CommitReferenceCommand) (*ReferenceDetail, error)
	ReleasePreviousReferences(ctx context.Context, cmd ReleasePreviousReferencesCommand) error
	GetReference(ctx context.Context, referenceID uint64, ownerUserID uint64) (*ReferenceDetail, error)
	GetBoundReference(ctx context.Context, cmd GetBoundReferenceCommand) (*ReferenceDetail, error)
	GetDownloadReference(ctx context.Context, referenceID uint64) (*DownloadObject, error)
	GetDownloadReferenceForOwner(ctx context.Context, referenceID uint64, ownerUserID uint64) (*DownloadObject, error)
	ReleaseReference(ctx context.Context, cmd ReleaseReferenceCommand) error
	ExpireTemporaryReferences(ctx context.Context, now time.Time, limit int) ([]ExpiredReference, error)
	FindUnreferencedObjects(ctx context.Context, limit int) ([]UnreferencedObject, error)
	MarkObjectDeleting(ctx context.Context, fileID uint64) (bool, error)
	MarkObjectDeleted(ctx context.Context, fileID uint64) error
	FindReusableObject(ctx context.Context, cmd FindReusableObjectCommand) (*ReusableObject, error)
	MarkObjectAvailable(ctx context.Context, fileID uint64) error
	MarkSessionFinished(ctx context.Context, sessionID uint64) error
	MarkUploadFailed(ctx context.Context, sessionID uint64) error
	MarkTusUploadFinished(ctx context.Context, cmd MarkTusUploadFinishedCommand) (*TusUploadDetail, error)
	MarkTusUploadCreated(ctx context.Context, cmd MarkTusUploadCreatedCommand) error
	FindExpiredTusUploads(ctx context.Context, now time.Time, limit int) ([]ExpiredTusUpload, error)
	MarkTusUploadAborted(ctx context.Context, sessionID uint64) error
	FindTusMetadataForCleanup(ctx context.Context, limit int) ([]TusMetadataObject, error)
	MarkTusMetadataCleaning(ctx context.Context, sessionID uint64) (bool, error)
	MarkTusMetadataCleaned(ctx context.Context, sessionID uint64) error
}

type CreateMultipartCommand struct {
	FileID       uint64
	Bucket       string
	ObjectKey    string
	OriginalName string
	ContentType  string
	SHA256       string
	Size         int64
	CreatedBy    uint64
	OwnerUserID  uint64
	Profile      string
	ExpiresAt    time.Time
}

type CreateMultipartResult struct {
	FileID      uint64
	ReferenceID uint64
	SessionID   uint64
	ExpiresAt   time.Time
}

type CreateTusCommand struct {
	FileID       uint64
	Bucket       string
	ObjectKey    string
	TusUploadID  string
	TusInfoKey   string
	TusPartKey   string
	OriginalName string
	ContentType  string
	SHA256       string
	Size         int64
	CreatedBy    uint64
	OwnerUserID  uint64
	Profile      string
	ExpiresAt    time.Time
}

type CreateTusResult struct {
	FileID      uint64
	ReferenceID uint64
	SessionID   uint64
	ObjectKey   string
	TusUploadID string
	TusInfoKey  string
	TusPartKey  string
	ExpiresAt   time.Time
}

type MarkTusUploadFinishedCommand struct {
	TusUploadID string
	ObjectKey   string
}

type MarkTusUploadCreatedCommand struct {
	TusUploadID string
	ObjectKey   string
}

type TusUploadDetail struct {
	FileID      uint64
	ReferenceID uint64
	SessionID   uint64
	ObjectKey   string
	TusInfoKey  string
	TusPartKey  string
}

type FindReusableObjectCommand struct {
	SHA256      string
	Size        int64
	OwnerUserID uint64
	Profile     string
}

type ReusableObject struct {
	FileID    uint64
	ObjectKey string
}

type CreateInstantReferenceCommand struct {
	FileID      uint64
	OwnerUserID uint64
	Profile     string
	ExpiresAt   time.Time
}

type CreateInstantReferenceResult struct {
	FileID      uint64
	ReferenceID uint64
	ObjectKey   string
	ExpiresAt   time.Time
}

type CommitReferenceCommand struct {
	ReferenceID          uint64
	OwnerUserID          uint64
	Profile              string
	Service              string
	ResourceType         string
	ResourceID           uint64
	FieldName            string
	DeferReleaseExisting bool
}

type ReleasePreviousReferencesCommand struct {
	ReferenceID  uint64
	Service      string
	ResourceType string
	ResourceID   uint64
	FieldName    string
}

type GetBoundReferenceCommand struct {
	Service      string
	ResourceType string
	ResourceID   uint64
	FieldName    string
	Profile      string
}

type ReferenceDetail struct {
	ReferenceID uint64
	FileID      uint64
	OwnerUserID uint64
	Profile     string
	Status      string
	ObjectKey   string
	ExpiresAt   time.Time
}

type DownloadObject struct {
	ReferenceID     uint64
	FileID          uint64
	OwnerUserID     uint64
	Bucket          string
	ObjectKey       string
	OriginalName    string
	ContentType     string
	Size            int64
	Profile         string
	ReferenceStatus string
	ObjectStatus    string
	ExpiresAt       time.Time
	AvailableAt     *time.Time
}

type ReleaseReferenceCommand struct {
	ReferenceID uint64
	OwnerUserID uint64
}

type ExpiredReference struct {
	ReferenceID uint64
	FileID      uint64
}

type UnreferencedObject struct {
	FileID    uint64
	ObjectKey string
}

type TusMetadataObject struct {
	SessionID  uint64
	FileID     uint64
	TusInfoKey string
	TusPartKey string
}

type ExpiredTusUpload struct {
	SessionID   uint64
	FileID      uint64
	ReferenceID uint64
	TusUploadID string
	ObjectKey   string
}
