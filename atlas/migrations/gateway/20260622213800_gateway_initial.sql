-- Create "api" table
CREATE TABLE `api` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT "id",
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `signcode` varchar(255) NOT NULL DEFAULT "" COMMENT "接口编码,md5(path+method)",
  `name` varchar(255) NOT NULL DEFAULT "" COMMENT "接口名称",
  `path` varchar(255) NOT NULL DEFAULT "" COMMENT "gRPC 服务名,casbin obj",
  `method` varchar(255) NOT NULL DEFAULT "" COMMENT "gRPC 方法名,casbin act",
  PRIMARY KEY (`id`),
  INDEX `idx_api_created_at` (`created_at`),
  INDEX `idx_api_deleted_at` (`deleted_at`),
  INDEX `idx_api_method` (`method`),
  INDEX `idx_api_name` (`name`),
  INDEX `idx_api_path` (`path`),
  UNIQUE INDEX `idx_api_signcode` (`signcode`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
