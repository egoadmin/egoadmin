table "auth_crypto_key" {
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
  column "key_id" {
    null    = false
    type    = varchar(64)
    default = ""
    comment = "密钥标识"
  }
  column "algorithm" {
    null    = false
    type    = varchar(64)
    default = ""
    comment = "算法"
  }
  column "public_key_pem" {
    null    = false
    type    = text
    comment = "公钥PEM"
  }
  column "private_key_pem" {
    null    = false
    type    = text
    comment = "私钥PEM"
  }
  column "status" {
    null    = false
    type    = int
    default = 1
    comment = "状态,1启用,2退役"
  }
  column "remark" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "备注"
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_auth_crypto_key_created_at" {
    columns = [column.created_at]
  }
  index "idx_auth_crypto_key_deleted_at" {
    columns = [column.deleted_at]
  }
  index "idx_auth_crypto_key_key_id" {
    unique  = true
    columns = [column.key_id]
  }
  index "idx_auth_crypto_key_status" {
    columns = [column.status]
  }
}
table "casbin_rule" {
  schema = schema.dev
  column "id" {
    null           = false
    type           = bigint
    unsigned       = true
    auto_increment = true
  }
  column "ptype" {
    null = true
    type = varchar(100)
  }
  column "v0" {
    null = true
    type = varchar(100)
  }
  column "v1" {
    null = true
    type = varchar(100)
  }
  column "v2" {
    null = true
    type = varchar(100)
  }
  column "v3" {
    null = true
    type = varchar(100)
  }
  column "v4" {
    null = true
    type = varchar(100)
  }
  column "v5" {
    null = true
    type = varchar(100)
  }
  primary_key {
    columns = [column.id]
  }
}
table "config" {
  schema = schema.dev
  column "ckey" {
    null    = false
    type    = varchar(255)
    comment = "键"
  }
  column "value" {
    null    = true
    type    = text
    comment = "值"
  }
  primary_key {
    columns = [column.ckey]
  }
}
table "dept" {
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
  column "code" {
    null    = false
    type    = varchar(500)
    default = ""
    comment = "部门编号[节点路径(节点id拼接)]"
  }
  column "parent_id" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "上级组织ID"
  }
  column "dept_name" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "组织名称"
  }
  column "leader" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "组织负责人"
  }
  column "phone" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "联系电话"
  }
  column "email" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "邮箱"
  }
  column "remark" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "备注"
  }
  column "priority" {
    null    = false
    type    = int
    default = 0
    comment = "排序,当前层级的排序"
  }
  column "status" {
    null    = false
    type    = int
    default = 1
    comment = "状态,1正常,2禁用"
  }
  column "level" {
    null    = false
    type    = int
    default = 1
    comment = "组织层级"
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_dept_code" {
    unique  = true
    columns = [column.code]
  }
  index "idx_dept_created_at" {
    columns = [column.created_at]
  }
  index "idx_dept_deleted_at" {
    columns = [column.deleted_at]
  }
  index "idx_dept_dept_name" {
    columns = [column.dept_name]
  }
  index "idx_dept_parent_id" {
    columns = [column.parent_id]
  }
}
table "role" {
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
  column "name" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "分组名称,角色"
  }
  column "typ" {
    null    = false
    type    = int
    default = 1
    comment = "类型: 1平台角色"
  }
  column "built_in" {
    null    = false
    type    = int
    default = 2
    comment = "是否内置角色,1内置角色,2普通角色"
  }
  column "data_perm" {
    null    = false
    type    = int
    default = 1
    comment = "数据权限: 1全部数据权限,2用户所属组织及子组织数据权限,3用户所属组织自身数据权限,4仅用户自身数据权限"
  }
  column "owner_user_id" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "角色创建者用户id"
  }
  column "owner_dept_id" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "角色归属组织id"
  }
  column "uses" {
    null    = false
    type    = text
    comment = "角色可用功能"
  }
  column "view_menus" {
    null    = false
    type    = text
    comment = "页面菜单"
  }
  column "desc" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "描述"
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_role_created_at" {
    columns = [column.created_at]
  }
  index "idx_role_deleted_at" {
    columns = [column.deleted_at]
  }
  index "idx_role_name" {
    columns = [column.name]
  }
  index "idx_role_owner_dept_id" {
    columns = [column.owner_dept_id]
  }
  index "idx_role_owner_user_id" {
    columns = [column.owner_user_id]
  }
}
table "role_permission_policy" {
  schema = schema.dev
  column "role_model_id" {
    null     = false
    type     = bigint
    unsigned = true
    comment  = "角色ID"
  }
  column "service" {
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
    columns = [column.role_model_id, column.service, column.method]
  }
}
table "sys_log" {
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
  column "user_id" {
    null    = false
    type    = varchar(50)
    default = ""
    comment = "用户id"
  }
  column "user_id_u64" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "用户id数值列"
  }
  column "username" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "用户名"
  }
  column "dept_id" {
    null    = false
    type    = varchar(50)
    default = ""
    comment = "用户部门id"
  }
  column "dept_id_u64" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "用户部门id数值列"
  }
  column "dept_name" {
    null    = false
    type    = varchar(1000)
    default = ""
    comment = "用户部门全称"
  }
  column "typ" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "操作类型"
  }
  column "module_name" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "模块名"
  }
  column "title" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "标题，如创建用户"
  }
  column "url" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "访问链接"
  }
  column "method" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "请求方法,如GET,POST等"
  }
  column "client_ip" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "客户端ip"
  }
  column "params" {
    null    = false
    type    = text
    comment = "请求参数"
  }
  column "remark" {
    null    = true
    type    = text
    comment = "备注"
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_sys_log_created_at" {
    columns = [column.created_at]
  }
  index "idx_sys_log_deleted_at" {
    columns = [column.deleted_at]
  }
  index "idx_sys_log_dept_id_u64_created_id" {
    columns = [column.dept_id_u64]
  }
  index "idx_sys_log_user_id_u64_created_id" {
    columns = [column.user_id_u64]
  }
}
table "user" {
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
  column "built_in" {
    null    = false
    type    = int
    default = 2
    comment = "是否内置用户,1内置用户,2普通用户"
  }
  column "username" {
    null    = false
    type    = varchar(255)
    comment = "用户名"
  }
  column "password" {
    null    = false
    type    = varchar(255)
    comment = "用户密码"
  }
  column "salt" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "用户密码盐"
  }
  column "name" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "姓名"
  }
  column "avatar" {
    null    = true
    type    = varchar(255)
    comment = "头像"
  }
  column "phone" {
    null    = true
    type    = varchar(255)
    comment = "手机号,中国手机不带国家代码，国际手机号格式为：国家代码-手机号"
  }
  column "email" {
    null    = true
    type    = varchar(255)
    comment = "邮箱"
  }
  column "realname" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "真实姓名"
  }
  column "nickname" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "昵称"
  }
  column "gender" {
    null    = true
    type    = tinyint
    comment = "性别,0:保密,1:男,2:女"
  }
  column "birthday" {
    null    = true
    type    = date
    default = "1970-01-01"
    comment = "生日"
  }
  column "first_login" {
    null    = false
    type    = int
    default = 1
    comment = "是否首次登录,1:是,2:不是"
  }
  column "last_login_ip" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "最后登录IP"
  }
  column "last_login_at" {
    null    = true
    type    = datetime
    default = sql("CURRENT_TIMESTAMP")
    comment = "最后登录时间"
  }
  column "user_status" {
    null    = false
    type    = int
    default = 1
    comment = "用户状态,1有效,2无效"
  }
  column "user_type" {
    null    = false
    type    = int
    comment = "用户类型,1:平台用户"
  }
  column "user_online" {
    null    = false
    type    = int
    default = 2
    comment = "用户在线状态,1:在线,2:不在线"
  }
  column "heartbeat_time" {
    null    = true
    type    = datetime
    default = sql("CURRENT_TIMESTAMP")
    comment = "心跳时间"
  }
  column "remark" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "备注"
  }
  column "dept_id" {
    null     = false
    type     = bigint
    default  = 0
    unsigned = true
    comment  = "组织id"
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_user_created_at" {
    columns = [column.created_at]
  }
  index "idx_user_deleted_at" {
    columns = [column.deleted_at]
  }
  index "idx_user_deptid" {
    columns = [column.dept_id]
  }
  index "idx_user_email" {
    unique  = true
    columns = [column.email]
  }
  index "idx_user_phone" {
    unique  = true
    columns = [column.phone]
  }
  index "idx_user_username" {
    unique  = true
    columns = [column.username]
  }
}
table "user_role" {
  schema = schema.dev
  column "user_model_id" {
    null     = false
    type     = bigint
    unsigned = true
  }
  column "role_model_id" {
    null     = false
    type     = bigint
    unsigned = true
  }
  primary_key {
    columns = [column.user_model_id, column.role_model_id]
  }
}
schema "dev" {
  charset = "utf8mb4"
  collate = "utf8mb4_0900_ai_ci"
}
