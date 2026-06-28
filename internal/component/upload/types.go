package upload

import "time"

const (
	DefaultProfile = "default"

	ObjectStatusUploading = "uploading"
	ObjectStatusAvailable = "available"
	ObjectStatusDeleting  = "deleting"
	ObjectStatusDeleted   = "deleted"

	ReferenceStatusTemporary = "temporary"
	ReferenceStatusBound     = "bound"
	ReferenceStatusReleased  = "released"
	ReferenceStatusExpired   = "expired"

	SessionTypeMultipart = "multipart"
	SessionTypeTus       = "tus"

	SessionStatusCreating         = "creating"
	SessionStatusUploading        = "uploading"
	SessionStatusFinished         = "finished"
	SessionStatusMetadataCleaning = "metadata_cleaning"
	SessionStatusMetadataCleaned  = "metadata_cleaned"
	SessionStatusFailed           = "failed"
	SessionStatusAborted          = "aborted"
)

const (
	ReuseScopeOwner  = "owner"
	ReuseScopePublic = "public"
)

type FileObject struct {
	ID           uint64
	Bucket       string
	ObjectKey    string
	SHA256       string
	Size         int64
	ContentType  string
	OriginalName string
	Status       string
	CreatedBy    uint64
	CreatedAt    time.Time
	AvailableAt  *time.Time
	DeletedAt    *time.Time
}

type FileReference struct {
	ID           uint64
	FileID       uint64
	OwnerUserID  uint64
	Profile      string
	Service      string
	ResourceType string
	ResourceID   uint64
	FieldName    string
	Status       string
	ExpiresAt    time.Time
	CreatedAt    time.Time
	BoundAt      *time.Time
	ReleasedAt   *time.Time
}

type UploadSession struct {
	ID                uint64
	UploadType        string
	TusUploadID       string
	FileID            uint64
	ReferenceID       uint64
	ObjectKey         string
	TusInfoKey        string
	TusPartKey        string
	Status            string
	CreatedAt         time.Time
	FinishedAt        *time.Time
	MetadataCleanedAt *time.Time
	ExpiresAt         time.Time
}

type UploadResult struct {
	FileID      uint64
	ReferenceID uint64
	SessionID   uint64
	Profile     string
	ObjectKey   string
	URL         string
	ExpiresAt   time.Time
}

type InstantResult struct {
	Hit          bool
	ShouldUpload bool
	FileID       string
	ReferenceID  string
	Profile      string
	URL          string
	ExpiresAt    time.Time
}

type ProfileInfo struct {
	Name              string   `json:"name"`
	MaxSize           int64    `json:"maxSize"`
	TTLSeconds        int64    `json:"ttlSeconds"`
	AllowedExtensions []string `json:"allowedExtensions,omitempty"`
	AllowedMimeTypes  []string `json:"allowedMimeTypes,omitempty"`
	TusRequired       bool     `json:"tusRequired"`
	MaxCount          int      `json:"maxCount,omitempty"`
	InstantEnabled    bool     `json:"instantEnabled"`
}

type ReferenceBinding struct {
	ReferenceID          uint64
	OwnerUserID          uint64
	Profile              string
	Service              string
	ResourceType         string
	ResourceID           uint64
	FieldName            string
	DeferReleaseExisting bool
}
