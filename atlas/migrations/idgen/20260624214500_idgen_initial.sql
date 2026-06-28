-- Create "idgen_segment" table
CREATE TABLE `idgen_segment` (
  `namespace` varchar(64) NOT NULL DEFAULT "" COMMENT "命名空间",
  `name` varchar(128) NOT NULL DEFAULT "" COMMENT "业务名称",
  `next_id` bigint NOT NULL DEFAULT 1 COMMENT "下一个可分配ID",
  `step` bigint NOT NULL DEFAULT 10000 COMMENT "基础步长",
  `min_step` bigint NOT NULL DEFAULT 10000 COMMENT "最小步长",
  `max_step` bigint NOT NULL DEFAULT 100000000 COMMENT "最大步长",
  `status` int NOT NULL DEFAULT 1 COMMENT "状态,1启用,2禁用",
  `last_step` bigint NOT NULL DEFAULT 0 COMMENT "上次实际领取步长",
  `description` varchar(255) NOT NULL DEFAULT "" COMMENT "描述",
  `last_fetch_at` datetime(3) NULL COMMENT "上次领取号段时间",
  `created_at` datetime(3) NULL COMMENT "创建时间",
  `updated_at` datetime(3) NULL COMMENT "更新时间",
  PRIMARY KEY (`namespace`, `name`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "idgen_machine_lease" table
CREATE TABLE `idgen_machine_lease` (
  `namespace` varchar(64) NOT NULL DEFAULT "" COMMENT "租约命名空间",
  `machine_id` int NOT NULL COMMENT "机器号",
  `instance_id` varchar(255) NOT NULL DEFAULT "" COMMENT "实例ID",
  `session_id` varchar(64) NOT NULL DEFAULT "" COMMENT "租约会话ID",
  `ttl_millis` bigint NOT NULL DEFAULT 30000 COMMENT "租约TTL毫秒",
  `renew_millis` bigint NOT NULL DEFAULT 10000 COMMENT "建议续租间隔毫秒",
  `expires_at` datetime(3) NOT NULL COMMENT "过期时间",
  `last_renewed_at` datetime(3) NOT NULL COMMENT "最近续租时间",
  `created_at` datetime(3) NULL COMMENT "创建时间",
  `updated_at` datetime(3) NULL COMMENT "更新时间",
  PRIMARY KEY (`namespace`, `machine_id`),
  INDEX `idx_idgen_machine_instance` (`instance_id`),
  INDEX `idx_idgen_machine_lease_expires_at` (`expires_at`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
