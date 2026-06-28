variable "service" {
  type    = string
  default = "gateway"
}

variable "dialect" {
  type    = string
  default = "mysql"
}

variable "dev_url" {
  type    = string
  default = ""
}

locals {
  default_dev_url = {
    mysql     = "docker://mysql/8/dev"
    postgres  = "docker://postgres/15/dev?search_path=public"
    sqlite    = "sqlite://file::memory:?cache=shared"
    sqlserver = "docker://sqlserver/2022-latest"
  }[var.dialect]

  dev_url = var.dev_url != "" ? var.dev_url : local.default_dev_url

  migration_dir = var.dialect == "mysql" ? "file://atlas/migrations/${var.service}" : "file://atlas/migrations/${var.dialect}/${var.service}"
}

data "external_schema" "gorm" {
  program = [
    "go",
    "run",
    "-mod=mod",
    "./tools/atlasloader",
    "--service",
    var.service,
    "--dialect",
    var.dialect,
  ]
}

env "gorm" {
  src = data.external_schema.gorm.url
  dev = local.dev_url

  migration {
    dir = local.migration_dir
    exclude = ["atlas_schema_revisions"]
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

env "local" {
  url = getenv("ATLAS_URL")
  dev = local.dev_url

  migration {
    dir = local.migration_dir
    exclude = ["atlas_schema_revisions"]
  }
}

env "schema" {
  src = data.external_schema.gorm.url
  dev = local.dev_url
}
