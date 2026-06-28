table "idgen_machine_lease" {
  schema = schema.dev
  column "namespace" {
    null    = false
    type    = varchar(64)
    default = ""
    comment = "租约命名空间"
  }
  column "machine_id" {
    null    = false
    type    = int
    comment = "机器号"
  }
  column "instance_id" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "实例ID"
  }
  column "session_id" {
    null    = false
    type    = varchar(64)
    default = ""
    comment = "租约会话ID"
  }
  column "ttl_millis" {
    null    = false
    type    = bigint
    default = 30000
    comment = "租约TTL毫秒"
  }
  column "renew_millis" {
    null    = false
    type    = bigint
    default = 10000
    comment = "建议续租间隔毫秒"
  }
  column "expires_at" {
    null    = false
    type    = datetime(3)
    comment = "过期时间"
  }
  column "last_renewed_at" {
    null    = false
    type    = datetime(3)
    comment = "最近续租时间"
  }
  column "created_at" {
    null    = true
    type    = datetime(3)
    comment = "创建时间"
  }
  column "updated_at" {
    null    = true
    type    = datetime(3)
    comment = "更新时间"
  }
  primary_key {
    columns = [column.namespace, column.machine_id]
  }
  index "idx_idgen_machine_instance" {
    columns = [column.instance_id]
  }
  index "idx_idgen_machine_lease_expires_at" {
    columns = [column.expires_at]
  }
}
table "idgen_segment" {
  schema = schema.dev
  column "namespace" {
    null    = false
    type    = varchar(64)
    default = ""
    comment = "命名空间"
  }
  column "name" {
    null    = false
    type    = varchar(128)
    default = ""
    comment = "业务名称"
  }
  column "next_id" {
    null    = false
    type    = bigint
    default = 1
    comment = "下一个可分配ID"
  }
  column "step" {
    null    = false
    type    = bigint
    default = 10000
    comment = "基础步长"
  }
  column "min_step" {
    null    = false
    type    = bigint
    default = 10000
    comment = "最小步长"
  }
  column "max_step" {
    null    = false
    type    = bigint
    default = 100000000
    comment = "最大步长"
  }
  column "status" {
    null    = false
    type    = int
    default = 1
    comment = "状态,1启用,2禁用"
  }
  column "last_step" {
    null    = false
    type    = bigint
    default = 0
    comment = "上次实际领取步长"
  }
  column "description" {
    null    = false
    type    = varchar(255)
    default = ""
    comment = "描述"
  }
  column "last_fetch_at" {
    null    = true
    type    = datetime(3)
    comment = "上次领取号段时间"
  }
  column "created_at" {
    null    = true
    type    = datetime(3)
    comment = "创建时间"
  }
  column "updated_at" {
    null    = true
    type    = datetime(3)
    comment = "更新时间"
  }
  primary_key {
    columns = [column.namespace, column.name]
  }
}
schema "dev" {
  charset = "utf8mb4"
  collate = "utf8mb4_0900_ai_ci"
}
