-- Modify "role" table
ALTER TABLE `role`
  ADD COLUMN `owner_user_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "角色创建者用户id" AFTER `data_perm`,
  ADD COLUMN `owner_dept_id` bigint unsigned NOT NULL DEFAULT 0 COMMENT "角色归属组织id" AFTER `owner_user_id`,
  ADD INDEX `idx_role_data_perm` (`data_perm`),
  ADD INDEX `idx_role_owner_user_id` (`owner_user_id`),
  ADD INDEX `idx_role_owner_dept_id` (`owner_dept_id`);

-- Modify "user" table
ALTER TABLE `user`
  ADD INDEX `idx_user_online_dept_id_id` (`user_online`, `dept_id`, `id`);

-- Modify "user_role" table
ALTER TABLE `user_role`
  ADD INDEX `idx_user_role_role_model_id_user_model_id` (`role_model_id`, `user_model_id`);

-- Modify "sys_log" table
ALTER TABLE `sys_log`
  ADD COLUMN `user_id_u64` bigint unsigned NOT NULL DEFAULT 0 COMMENT "用户id数值列" AFTER `user_id`,
  ADD COLUMN `dept_id_u64` bigint unsigned NOT NULL DEFAULT 0 COMMENT "用户部门id数值列" AFTER `dept_id`,
  ADD INDEX `idx_sys_log_user_id_u64_created_id` (`user_id_u64`, `created_at`, `id`),
  ADD INDEX `idx_sys_log_dept_id_u64_created_id` (`dept_id_u64`, `created_at`, `id`);
