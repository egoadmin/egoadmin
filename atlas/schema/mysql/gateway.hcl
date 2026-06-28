table "api" {
  schema = schema.dev
  column "id" {
    null           = false
    type           = bigint
    unsigned       = true
    comment        = "id"
    auto_increment = true
  }
  column "created_at" {
    null = true
    type = datetime(3)
  }
  column "updated_at" {
    null = true
    type = datetime(3)
  }
  column "deleted_at" {
    null = true
    type = datetime(3)
  }
  column "signcode" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "接口编码,md5(path+method)"
  }
  column "name" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "接口名称"
  }
  column "path" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "gRPC 服务名,casbin obj"
  }
  column "method" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "gRPC 方法名,casbin act"
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_api_created_at" {
    columns = [column.created_at]
  }
  index "idx_api_deleted_at" {
    columns = [column.deleted_at]
  }
  index "idx_api_method" {
    columns = [column.method]
  }
  index "idx_api_name" {
    columns = [column.name]
  }
  index "idx_api_path" {
    columns = [column.path]
  }
  index "idx_api_signcode" {
    unique  = true
    columns = [column.signcode]
  }
}
table "file_object" {
  schema = schema.dev
  column "id" {
    null           = false
    type           = bigint
    unsigned       = true
    comment        = "id"
    auto_increment = true
  }
  column "created_at" {
    null = true
    type = datetime(3)
  }
  column "updated_at" {
    null = true
    type = datetime(3)
  }
  column "deleted_at" {
    null = true
    type = datetime(3)
  }
  column "bucket" {
    null    = false
    type    = varchar(128)
    default = ""
    comment = "对象存储桶"
  }
  column "object_key" {
    null    = false
    type    = varchar(512)
    default = ""
    comment = "对象key"
  }
  column "sha256" {
    null    = false
    type    = varchar(64)
    default = ""
    comment = "sha256"
  }
  column "size" {
    null    = false
    type    = bigint
    default = 0
    comment = "文件大小"
  }
  column "content_type" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "content-type"
  }
  column "original_name" {
    null    = false
    type    = varchar(512)
    default = ""
    comment = "原始文件名"
  }
  column "status" {
    null    = false
    type    = varchar(32)
    default = ""
    comment = "状态"
  }
  column "created_by" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "创建用户"
  }
  column "available_at" {
    null    = true
    type    = datetime(3)
    comment = "可用时间"
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_file_object_created_at" {
    columns = [column.created_at]
  }
  index "idx_file_object_created_by" {
    columns = [column.created_by]
  }
  index "idx_file_object_deleted_at" {
    columns = [column.deleted_at]
  }
  index "idx_file_object_hash_size" {
    columns = [column.sha256, column.size]
  }
  index "idx_file_object_status_created" {
    columns = [column.status]
  }
  index "uniq_file_object_bucket_key" {
    unique  = true
    columns = [column.bucket, column.object_key]
  }
}
table "file_reference" {
  schema = schema.dev
  column "id" {
    null           = false
    type           = bigint
    unsigned       = true
    comment        = "id"
    auto_increment = true
  }
  column "created_at" {
    null = true
    type = datetime(3)
  }
  column "updated_at" {
    null = true
    type = datetime(3)
  }
  column "deleted_at" {
    null = true
    type = datetime(3)
  }
  column "file_id" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "文件ID"
  }
  column "owner_user_id" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "所属用户"
  }
  column "profile" {
    null    = false
    type    = varchar(64)
    default = ""
    comment = "上传策略"
  }
  column "service" {
    null    = false
    type    = varchar(128)
    default = ""
    comment = "绑定服务"
  }
  column "resource_type" {
    null    = false
    type    = varchar(128)
    default = ""
    comment = "资源类型"
  }
  column "resource_id" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "资源ID"
  }
  column "field_name" {
    null    = false
    type    = varchar(128)
    default = ""
    comment = "字段名"
  }
  column "status" {
    null    = false
    type    = varchar(32)
    default = ""
    comment = "状态"
  }
  column "expires_at" {
    null    = false
    type    = datetime(3)
    comment = "过期时间"
  }
  column "bound_at" {
    null    = true
    type    = datetime(3)
    comment = "绑定时间"
  }
  column "released_at" {
    null    = true
    type    = datetime(3)
    comment = "释放时间"
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_file_reference_binding" {
    columns = [column.service, column.resource_type, column.resource_id, column.field_name]
  }
  index "idx_file_reference_created_at" {
    columns = [column.created_at]
  }
  index "idx_file_reference_deleted_at" {
    columns = [column.deleted_at]
  }
  index "idx_file_reference_file_status" {
    columns = [column.file_id, column.status]
  }
  index "idx_file_reference_owner_profile_status" {
    columns = [column.owner_user_id, column.profile, column.status]
  }
  index "idx_file_reference_status_expires" {
    columns = [column.status, column.expires_at]
  }
}
table "upload_session" {
  schema = schema.dev
  column "id" {
    null           = false
    type           = bigint
    unsigned       = true
    comment        = "id"
    auto_increment = true
  }
  column "created_at" {
    null = true
    type = datetime(3)
  }
  column "updated_at" {
    null = true
    type = datetime(3)
  }
  column "deleted_at" {
    null = true
    type = datetime(3)
  }
  column "upload_type" {
    null    = false
    type    = varchar(32)
    default = ""
    comment = "上传类型"
  }
  column "tus_upload_id" {
    null    = false
    type    = varchar(512)
    default = ""
    comment = "tus upload id"
  }
  column "file_id" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "文件ID"
  }
  column "reference_id" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "引用ID"
  }
  column "object_key" {
    null    = false
    type    = varchar(512)
    default = ""
    comment = "对象key"
  }
  column "tus_info_key" {
    null    = false
    type    = varchar(512)
    default = ""
    comment = "tus info key"
  }
  column "tus_part_key" {
    null    = false
    type    = varchar(512)
    default = ""
    comment = "tus part key"
  }
  column "status" {
    null    = false
    type    = varchar(32)
    default = ""
    comment = "状态"
  }
  column "finished_at" {
    null    = true
    type    = datetime(3)
    comment = "完成时间"
  }
  column "metadata_cleaned_at" {
    null    = true
    type    = datetime(3)
    comment = "元数据清理时间"
  }
  column "expires_at" {
    null    = false
    type    = datetime(3)
    comment = "过期时间"
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_upload_session_created_at" {
    columns = [column.created_at]
  }
  index "idx_upload_session_deleted_at" {
    columns = [column.deleted_at]
  }
  index "idx_upload_session_finished_status" {
    columns = [column.status, column.finished_at]
  }
  index "idx_upload_session_status_expires" {
    columns = [column.status, column.expires_at]
  }
  index "idx_upload_session_tus_upload_id" {
    columns = [column.tus_upload_id]
  }
}
schema "dev" {
  charset = "utf8mb4"
  collate = "utf8mb4_0900_ai_ci"
}
