package mysql

import (
	"time"

	uploadcomponent "github.com/egoadmin/egoadmin/internal/component/upload"
	platformmysql "github.com/egoadmin/egoadmin/internal/platform/database/mysql"
	"github.com/egoadmin/elib/pkg/util/xorm"
	"gorm.io/gorm"
)

type fileObjectModel struct {
	xorm.Model
	Bucket       string     `gorm:"uniqueIndex:uniq_file_object_bucket_key;type:varchar(128);not null;default:'';comment:对象存储桶"`
	ObjectKey    string     `gorm:"uniqueIndex:uniq_file_object_bucket_key;type:varchar(512);not null;default:'';comment:对象key"`
	SHA256       string     `gorm:"index:idx_file_object_hash_size;type:varchar(64);not null;default:'';comment:sha256"`
	Size         int64      `gorm:"index:idx_file_object_hash_size;not null;default:0;comment:文件大小"`
	ContentType  string     `gorm:"type:varchar(255);not null;default:'';comment:content-type"`
	OriginalName string     `gorm:"type:varchar(512);not null;default:'';comment:原始文件名"`
	Status       string     `gorm:"index:idx_file_object_status_created;type:varchar(32);not null;default:'';comment:状态"`
	CreatedBy    uint64     `gorm:"index;not null;default:0;comment:创建用户"`
	AvailableAt  *time.Time `gorm:"comment:可用时间"`
}

func (fileObjectModel) TableName() string {
	return "file_object"
}

func (m *fileObjectModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

func (m *fileObjectModel) BeforeCreate(tx *gorm.DB) error {
	return platformmysql.SetID(m)
}

type fileReferenceModel struct {
	xorm.Model
	FileID       uint64     `gorm:"index:idx_file_reference_file_status;not null;default:0;comment:文件ID"`
	OwnerUserID  uint64     `gorm:"index:idx_file_reference_owner_profile_status;not null;default:0;comment:所属用户"`
	Profile      string     `gorm:"index:idx_file_reference_owner_profile_status;type:varchar(64);not null;default:'';comment:上传策略"`
	Service      string     `gorm:"index:idx_file_reference_binding;type:varchar(128);not null;default:'';comment:绑定服务"`
	ResourceType string     `gorm:"index:idx_file_reference_binding;type:varchar(128);not null;default:'';comment:资源类型"`
	ResourceID   uint64     `gorm:"index:idx_file_reference_binding;not null;default:0;comment:资源ID"`
	FieldName    string     `gorm:"index:idx_file_reference_binding;type:varchar(128);not null;default:'';comment:字段名"`
	Status       string     `gorm:"index:idx_file_reference_owner_profile_status;index:idx_file_reference_status_expires;index:idx_file_reference_file_status;type:varchar(32);not null;default:'';comment:状态"`
	ExpiresAt    time.Time  `gorm:"index:idx_file_reference_status_expires;not null;comment:过期时间"`
	BoundAt      *time.Time `gorm:"comment:绑定时间"`
	ReleasedAt   *time.Time `gorm:"comment:释放时间"`
}

func (fileReferenceModel) TableName() string {
	return "file_reference"
}

func (m *fileReferenceModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

func (m *fileReferenceModel) BeforeCreate(tx *gorm.DB) error {
	return platformmysql.SetID(m)
}

type uploadSessionModel struct {
	xorm.Model
	UploadType        string     `gorm:"type:varchar(32);not null;default:'';comment:上传类型"`
	TusUploadID       string     `gorm:"index;type:varchar(512);not null;default:'';comment:tus upload id"`
	FileID            uint64     `gorm:"not null;default:0;comment:文件ID"`
	ReferenceID       uint64     `gorm:"not null;default:0;comment:引用ID"`
	ObjectKey         string     `gorm:"type:varchar(512);not null;default:'';comment:对象key"`
	TusInfoKey        string     `gorm:"type:varchar(512);not null;default:'';comment:tus info key"`
	TusPartKey        string     `gorm:"type:varchar(512);not null;default:'';comment:tus part key"`
	Status            string     `gorm:"index:idx_upload_session_status_expires;index:idx_upload_session_finished_status;type:varchar(32);not null;default:'';comment:状态"`
	FinishedAt        *time.Time `gorm:"index:idx_upload_session_finished_status;comment:完成时间"`
	MetadataCleanedAt *time.Time `gorm:"comment:元数据清理时间"`
	ExpiresAt         time.Time  `gorm:"index:idx_upload_session_status_expires;not null;comment:过期时间"`
}

func (uploadSessionModel) TableName() string {
	return "upload_session"
}

func (m *uploadSessionModel) SetID(id uint64) {
	if m.ID == 0 {
		m.ID = id
	}
}

func (m *uploadSessionModel) BeforeCreate(tx *gorm.DB) error {
	return platformmysql.SetID(m)
}

func fileObjectFromCommand(cmd uploadcomponent.CreateMultipartCommand) *fileObjectModel {
	return &fileObjectModel{
		Model:        xorm.Model{ID: cmd.FileID},
		Bucket:       cmd.Bucket,
		ObjectKey:    cmd.ObjectKey,
		SHA256:       cmd.SHA256,
		Size:         cmd.Size,
		ContentType:  cmd.ContentType,
		OriginalName: cmd.OriginalName,
		Status:       uploadcomponent.ObjectStatusUploading,
		CreatedBy:    cmd.CreatedBy,
	}
}

func fileObjectFromTusCommand(cmd uploadcomponent.CreateTusCommand) *fileObjectModel {
	return &fileObjectModel{
		Model:        xorm.Model{ID: cmd.FileID},
		Bucket:       cmd.Bucket,
		ObjectKey:    cmd.ObjectKey,
		SHA256:       cmd.SHA256,
		Size:         cmd.Size,
		ContentType:  cmd.ContentType,
		OriginalName: cmd.OriginalName,
		Status:       uploadcomponent.ObjectStatusUploading,
		CreatedBy:    cmd.CreatedBy,
	}
}
