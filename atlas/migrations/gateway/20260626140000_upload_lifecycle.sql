-- Create "file_object" table
CREATE TABLE `file_object` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT "id",
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `bucket` varchar(128) NOT NULL DEFAULT "" COMMENT "对象存储桶",
  `object_key` varchar(512) NOT NULL DEFAULT "" COMMENT "对象key",
  `sha256` varchar(64) NOT NULL DEFAULT "" COMMENT "sha256",
  `size` bigint NOT NULL DEFAULT 0 COMMENT "文件大小",
  `content_type` varchar(255) NOT NULL DEFAULT "" COMMENT "content-type",
  `original_name` varchar(512) NOT NULL DEFAULT "" COMMENT "原始文件名",
  `status` varchar(32) NOT NULL DEFAULT "" COMMENT "状态",
  `created_by` bigint unsigned NOT NULL DEFAULT 0 COMMENT "创建用户",
  `available_at` datetime(3) NULL COMMENT "可用时间",
  PRIMARY KEY (`id`),
  INDEX `idx_file_object_created_at` (`created_at`),
  INDEX `idx_file_object_deleted_at` (`deleted_at`),
  INDEX `idx_file_object_hash_size` (`sha256`, `size`),
  INDEX `idx_file_object_status_created` (`status`, `created_at`),
  INDEX `idx_file_object_created_by` (`created_by`),
  UNIQUE INDEX `uniq_file_object_bucket_key` (`bucket`, `object_key`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- Create "file_reference" table
CREATE TABLE `file_reference` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT "id",
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `file_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "文件ID",
  `owner_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "所属用户",
  `profile` varchar(64) NOT NULL DEFAULT "" COMMENT "上传策略",
  `service` varchar(128) NOT NULL DEFAULT "" COMMENT "绑定服务",
  `resource_type` varchar(128) NOT NULL DEFAULT "" COMMENT "资源类型",
  `resource_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "资源ID",
  `field_name` varchar(128) NOT NULL DEFAULT "" COMMENT "字段名",
  `status` varchar(32) NOT NULL DEFAULT "" COMMENT "状态",
  `expires_at` datetime(3) NOT NULL COMMENT "过期时间",
  `bound_at` datetime(3) NULL COMMENT "绑定时间",
  `released_at` datetime(3) NULL COMMENT "释放时间",
  PRIMARY KEY (`id`),
  INDEX `idx_file_reference_created_at` (`created_at`),
  INDEX `idx_file_reference_deleted_at` (`deleted_at`),
  INDEX `idx_file_reference_owner_profile_status` (`owner_user_id`, `profile`, `status`),
  INDEX `idx_file_reference_status_expires` (`status`, `expires_at`),
  INDEX `idx_file_reference_binding` (`service`, `resource_type`, `resource_id`, `field_name`),
  INDEX `idx_file_reference_file_status` (`file_id`, `status`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- Create "upload_session" table
CREATE TABLE `upload_session` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT "id",
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `upload_type` varchar(32) NOT NULL DEFAULT "" COMMENT "上传类型",
  `tus_upload_id` varchar(512) NOT NULL DEFAULT "" COMMENT "tus upload id",
  `file_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "文件ID",
  `reference_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "引用ID",
  `object_key` varchar(512) NOT NULL DEFAULT "" COMMENT "对象key",
  `tus_info_key` varchar(512) NOT NULL DEFAULT "" COMMENT "tus info key",
  `tus_part_key` varchar(512) NOT NULL DEFAULT "" COMMENT "tus part key",
  `status` varchar(32) NOT NULL DEFAULT "" COMMENT "状态",
  `finished_at` datetime(3) NULL COMMENT "完成时间",
  `metadata_cleaned_at` datetime(3) NULL COMMENT "元数据清理时间",
  `expires_at` datetime(3) NOT NULL COMMENT "过期时间",
  PRIMARY KEY (`id`),
  INDEX `idx_upload_session_created_at` (`created_at`),
  INDEX `idx_upload_session_deleted_at` (`deleted_at`),
  INDEX `idx_upload_session_tus_upload_id` (`tus_upload_id`),
  INDEX `idx_upload_session_status_expires` (`status`, `expires_at`),
  INDEX `idx_upload_session_finished_status` (`finished_at`, `status`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
