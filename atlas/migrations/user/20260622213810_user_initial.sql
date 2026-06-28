-- Create "auth_crypto_key" table
CREATE TABLE `auth_crypto_key` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT "id",
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `key_id` varchar(64) NOT NULL DEFAULT "" COMMENT "密钥标识",
  `algorithm` varchar(64) NOT NULL DEFAULT "" COMMENT "算法",
  `public_key_pem` text NOT NULL COMMENT "公钥PEM",
  `private_key_pem` text NOT NULL COMMENT "私钥PEM",
  `status` int NOT NULL DEFAULT 1 COMMENT "状态,1启用,2退役",
  `remark` varchar(255) NOT NULL DEFAULT "" COMMENT "备注",
  PRIMARY KEY (`id`),
  INDEX `idx_auth_crypto_key_created_at` (`created_at`),
  INDEX `idx_auth_crypto_key_deleted_at` (`deleted_at`),
  UNIQUE INDEX `idx_auth_crypto_key_key_id` (`key_id`),
  INDEX `idx_auth_crypto_key_status` (`status`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "casbin_rule" table
CREATE TABLE `casbin_rule` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `ptype` varchar(100) NULL,
  `v0` varchar(100) NULL,
  `v1` varchar(100) NULL,
  `v2` varchar(100) NULL,
  `v3` varchar(100) NULL,
  `v4` varchar(100) NULL,
  `v5` varchar(100) NULL,
  PRIMARY KEY (`id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "config" table
CREATE TABLE `config` (
  `ckey` varchar(255) NOT NULL COMMENT "键",
  `value` text NULL COMMENT "值",
  PRIMARY KEY (`ckey`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "dept" table
CREATE TABLE `dept` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT "id",
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `code` varchar(500) NOT NULL DEFAULT "" COMMENT "部门编号[节点路径(节点id拼接)]",
  `parent_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "上级组织ID",
  `dept_name` varchar(255) NOT NULL DEFAULT "" COMMENT "组织名称",
  `leader` varchar(255) NOT NULL DEFAULT "" COMMENT "组织负责人",
  `phone` varchar(255) NOT NULL DEFAULT "" COMMENT "联系电话",
  `email` varchar(255) NOT NULL DEFAULT "" COMMENT "邮箱",
  `remark` varchar(255) NOT NULL DEFAULT "" COMMENT "备注",
  `priority` int NOT NULL DEFAULT 0 COMMENT "排序,当前层级的排序",
  `status` int NOT NULL DEFAULT 1 COMMENT "状态,1正常,2禁用",
  `level` int NOT NULL DEFAULT 1 COMMENT "组织层级",
  PRIMARY KEY (`id`),
  UNIQUE INDEX `idx_dept_code` (`code`),
  INDEX `idx_dept_created_at` (`created_at`),
  INDEX `idx_dept_deleted_at` (`deleted_at`),
  INDEX `idx_dept_dept_name` (`dept_name`),
  INDEX `idx_dept_parent_id` (`parent_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "role" table
CREATE TABLE `role` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT "id",
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `name` varchar(255) NOT NULL DEFAULT "" COMMENT "分组名称,角色",
  `typ` int NOT NULL DEFAULT 1 COMMENT "类型: 1平台角色",
  `built_in` int NOT NULL DEFAULT 2 COMMENT "是否内置角色,1内置角色,2普通角色",
  `data_perm` int NOT NULL DEFAULT 1 COMMENT "数据权限: 1全部数据权限,2用户所属组织及子组织数据权限,3用户所属组织自身数据权限,4仅用户自身数据权限",
  `uses` text NOT NULL COMMENT "角色可用功能",
  `view_menus` text NOT NULL COMMENT "页面菜单",
  `desc` varchar(255) NOT NULL DEFAULT "" COMMENT "描述",
  PRIMARY KEY (`id`),
  INDEX `idx_role_created_at` (`created_at`),
  INDEX `idx_role_deleted_at` (`deleted_at`),
  INDEX `idx_role_name` (`name`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "role_permission_policy" table
CREATE TABLE `role_permission_policy` (
  `role_model_id` bigint unsigned NOT NULL COMMENT "角色ID",
  `service` varchar(255) NOT NULL DEFAULT "" COMMENT "gRPC 服务名,casbin obj",
  `method` varchar(255) NOT NULL DEFAULT "" COMMENT "gRPC 方法名,casbin act",
  PRIMARY KEY (`role_model_id`, `service`, `method`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "sys_log" table
CREATE TABLE `sys_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT "id",
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `user_id` varchar(50) NOT NULL DEFAULT "" COMMENT "用户id",
  `username` varchar(255) NOT NULL DEFAULT "" COMMENT "用户名",
  `dept_id` varchar(50) NOT NULL DEFAULT "" COMMENT "用户部门id",
  `dept_name` varchar(1000) NOT NULL DEFAULT "" COMMENT "用户部门全称",
  `typ` varchar(255) NOT NULL DEFAULT "" COMMENT "操作类型",
  `module_name` varchar(255) NOT NULL DEFAULT "" COMMENT "模块名",
  `title` varchar(255) NOT NULL DEFAULT "" COMMENT "标题，如创建用户",
  `url` varchar(255) NOT NULL DEFAULT "" COMMENT "访问链接",
  `method` varchar(255) NOT NULL DEFAULT "" COMMENT "请求方法,如GET,POST等",
  `client_ip` varchar(255) NOT NULL DEFAULT "" COMMENT "客户端ip",
  `params` text NOT NULL COMMENT "请求参数",
  `remark` text NULL COMMENT "备注",
  PRIMARY KEY (`id`),
  INDEX `idx_sys_log_created_at` (`created_at`),
  INDEX `idx_sys_log_deleted_at` (`deleted_at`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "user" table
CREATE TABLE `user` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT "id",
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `built_in` int NOT NULL DEFAULT 2 COMMENT "是否内置用户,1内置用户,2普通用户",
  `username` varchar(255) NOT NULL COMMENT "用户名",
  `password` varchar(255) NOT NULL COMMENT "用户密码",
  `salt` varchar(255) NOT NULL DEFAULT "" COMMENT "用户密码盐",
  `name` varchar(255) NOT NULL DEFAULT "" COMMENT "姓名",
  `avatar` varchar(255) NULL COMMENT "头像",
  `phone` varchar(255) NULL COMMENT "手机号,中国手机不带国家代码，国际手机号格式为：国家代码-手机号",
  `email` varchar(255) NULL COMMENT "邮箱",
  `realname` varchar(255) NOT NULL DEFAULT "" COMMENT "真实姓名",
  `nickname` varchar(255) NOT NULL DEFAULT "" COMMENT "昵称",
  `gender` tinyint NULL COMMENT "性别,0:保密,1:男,2:女",
  `birthday` date NULL DEFAULT "1970-01-01" COMMENT "生日",
  `first_login` int NOT NULL DEFAULT 1 COMMENT "是否首次登录,1:是,2:不是",
  `last_login_ip` varchar(255) NOT NULL DEFAULT "" COMMENT "最后登录IP",
  `last_login_at` datetime NULL DEFAULT CURRENT_TIMESTAMP COMMENT "最后登录时间",
  `user_status` int NOT NULL DEFAULT 1 COMMENT "用户状态,1有效,2无效",
  `user_type` int NOT NULL COMMENT "用户类型,1:平台用户",
  `user_online` int NOT NULL DEFAULT 2 COMMENT "用户在线状态,1:在线,2:不在线",
  `heartbeat_time` datetime NULL DEFAULT CURRENT_TIMESTAMP COMMENT "心跳时间",
  `remark` varchar(255) NOT NULL DEFAULT "" COMMENT "备注",
  `dept_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "组织id",
  PRIMARY KEY (`id`),
  INDEX `idx_user_created_at` (`created_at`),
  INDEX `idx_user_deleted_at` (`deleted_at`),
  INDEX `idx_user_deptid` (`dept_id`),
  UNIQUE INDEX `idx_user_email` (`email`),
  UNIQUE INDEX `idx_user_phone` (`phone`),
  UNIQUE INDEX `idx_user_username` (`username`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
-- Create "user_role" table
CREATE TABLE `user_role` (
  `user_model_id` bigint unsigned NOT NULL,
  `role_model_id` bigint unsigned NOT NULL,
  PRIMARY KEY (`user_model_id`, `role_model_id`)
) CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
