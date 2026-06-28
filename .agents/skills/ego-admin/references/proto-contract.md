# Proto Contract

Read this before changing any `.proto` file, service/rpc/message/field comments, `tagger.tags`, OpenAPI/OpenAPIv2 documentation options, `google.api.http`, request/response messages, or `errors.proto`.

## Source Of Truth

Interfaces start in `api/proto`. Generated code under `api/gen/go` is output, not source. Current CoreAdmin proto examples live under `api/proto/user/v1/`; derived projects may add domains such as `line.v1`, `task.v1`, or `app.v1`.

## Complete Proto File Structure

For new or heavily revised proto files, check this structure:

1. `syntax = "proto3";`
2. `package <domain>.v1;`
3. Imports as needed:
   - `google/api/annotations.proto`
   - `google/api/field_behavior.proto`
   - `google/protobuf/timestamp.proto`
   - `protoc-gen-openapiv2/options/annotations.proto`
   - `tagger/tagger.proto`
   - same-domain or cross-domain proto files
4. `option go_package = "...";` when the project is not relying on Buf managed `go_package_prefix`.
5. File-level `openapiv2_swagger` for complete OpenAPI title, description, version, license, schemes, consumes, and produces.
6. Service-level `openapiv2_tag` for Chinese service description and full service name, for example `服务名: workspace.v1.WorkspaceService`.
7. Service definition.
8. RPC definitions with comments, `google.api.http`, `openapiv2_operation` tags, and operation-level auth documentation.
9. Request messages.
10. Response messages.
11. Entity/data messages.
12. Package-level `errors.proto` enum when business errors are needed.

CoreAdmin currently uses Buf managed `go_package_prefix` from `buf.gen.yaml`, so existing proto files may omit explicit `go_package`. Do not mechanically copy a derived project's module path into template code.

## Interface Design Workflow

Before writing code in controller/service/mysql, design the proto surface:

1. Name the package and service around the business domain, not a frontend page.
2. Define RPCs as gRPC methods: `AddX`, `UpdateX`, `DeleteX`, `GetX`, `GetXList`, `GetXAll`, or domain-specific verbs.
3. For every RPC, define separate request and response messages even if they are currently empty.
4. Define entity/data messages after request/response messages.
5. Decide which fields are request input, response output, both, or computed output.
6. Put runtime validation only on request messages or entity fields reused by requests.
7. Add `copier` tags for every DB/model naming mismatch before writing controller conversion.
8. Add OpenAPI documentation annotations at file, service, RPC, and required input field level before generation.
9. Add generated business errors in `errors.proto` for reusable domain failures.
10. Use comments to explain business meaning and enum values before relying on generated OpenAPI docs.
11. After generation, confirm controller, copier, mysql model methods, frontend types, permission API names, and OpenAPI output all match.

Do not let an existing database table dictate the public contract blindly. Proto is the interface; DB models are implementation.

## OpenAPI Documentation Contract

OpenAPI docs are part of the proto contract. Do not stop at comments and `validate` tags. For new or heavily revised proto files, write complete OpenAPI annotations before running generation.

Before writing auth/security documentation, inspect the backend auth implementation. Do not infer auth classification from RPC names.

### File-Level Swagger

Every new service proto should include a file-level `openapiv2_swagger` block:

```protobuf
option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_swagger) = {
  info: {
    title: "工作空间服务"
    description: "服务名: workspace.v1.WorkspaceService"
    version: "1.0"
    license: {
      name: "MIT"
    },
  },
  schemes: HTTP
  consumes: "application/json"
  produces: "application/json"
  security_definitions: {
    security: {
      key: "BearerAuth"
      value: {
        type: TYPE_API_KEY
        in: IN_HEADER
        name: "Authorization"
        description: "JWT 认证头。Swagger UI 中请输入完整值: Bearer <token>"
      }
    }
  }
};
```

Rules:

- `title` is the Chinese service/domain name.
- `description` must include the full gRPC service name as `服务名: <package>.<Service>`.
- Use `version: "1.0"` unless the project has a different documented versioning rule.
- Use `license.name: "MIT"` when no project-specific license is provided.
- Keep `schemes: HTTP`, `consumes: "application/json"`, and `produces: "application/json"` for current CoreAdmin-style JSON HTTP compatibility output.
- Define `BearerAuth` in `security_definitions` for docs and Swagger UI usage.
- Do not write file-level global `security`. Public RPCs such as login and captcha must not be shown as requiring Authorization.

### Service-Level Tag

Every service should include `openapiv2_tag`:

```protobuf
// WorkspaceService 工作空间服务
service WorkspaceService {
  option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_tag) = {
    description: "工作空间服务,服务名: workspace.v1.WorkspaceService"
  };
}
```

Rules:

- The description must include both Chinese service name and full service name.
- Keep the full service name synchronized with the proto `package` and `service`.

### RPC Operation

Every RPC should include `openapiv2_operation`:

```protobuf
// GetWorkspaceList 查询工作空间列表
rpc GetWorkspaceList(GetWorkspaceListRequest) returns (GetWorkspaceListResponse) {
  option (google.api.http) = {
    post: "/workspace.v1.WorkspaceService/GetWorkspaceList"
    body: "*"
  };
  option (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_operation) = {
    tags: "工作空间服务,服务名: workspace.v1.WorkspaceService"
    tags: "WorkspaceService"
    description: "认证: 需要 Authorization: Bearer <token>，并需要接口权限 WORKSPACE.V1.WORKSPACESERVICE/GETWORKSPACELIST。"
    security: {
      security_requirement: {
        key: "BearerAuth"
        value: {}
      }
    }
  };
}
```

Rules:

- The first `tags` entry should include the Chinese service name and `服务名: <package>.<Service>`.
- The second `tags` entry should be the service name, such as `WorkspaceService`.
- Keep the `google.api.http` mapping POST-only as described in the next section; do not copy RESTful verbs from examples.
- Write `description` with the correct auth meaning for the RPC.
- Add operation-level `security` only for non-public RPCs.

### OpenAPI Auth Documentation

OpenAPI auth documentation must match backend auth behavior.

Before documenting auth, inspect the owning service runtime:

- `internal/app/gateway/server/grpc_server.go`
- `internal/app/user/server/grpc_server.go`
- `openPack`: public interfaces, no login required, such as `Login` and `GetCaptcha`.
- `justLoginPack`: login-only interfaces, `Authorization: Bearer <accessToken>` required but Casbin skipped, such as `GetMenus`, `Logout`, `HeartBeatUser`, and `CenterService`.
- Default behavior: protected interfaces require `authsession` Bearer login and Casbin API permission.

Do not guess classification from method names. For example, a `GetX` RPC is not automatically public, and a `Login`-like name is not automatically public unless it is in `openPack`.

File-level rule:

- Define `BearerAuth` in `security_definitions`.
- Do not add global file-level `security`.

Operation-level rule:

- Public RPCs: do not add operation `security`.
- Login-only RPCs: add operation `security` for `BearerAuth`; document that no interface permission is required.
- Protected RPCs: add operation `security` for `BearerAuth`; document the exact Casbin/RegisterAPIs permission identifier.

Operation security block for login-only and protected RPCs:

```protobuf
security: {
  security_requirement: {
    key: "BearerAuth"
    value: {}
  }
}
```

RPC description wording:

- Public: `认证: 公开接口，无需 Authorization。`
- Login-only: `认证: 需要 Authorization: Bearer <token>，无需接口权限。`
- Protected: `认证: 需要 Authorization: Bearer <token>，并需要接口权限 USER.V1.ROLESERVICE/ADDROLE。`

Protected permission identifiers must match `RegisterAPIs` and Casbin:

```text
<UPPERCASE_FULL_SERVICE>/<UPPERCASE_METHOD>
```

Examples:

- `USER.V1.ROLESERVICE/ADDROLE`
- `WORKSPACE.V1.WORKSPACESERVICE/GETWORKSPACELIST`

Do not use HTTP paths, frontend route paths, menu IDs, lower-case service names, or method-only names as permission identifiers in OpenAPI descriptions.

### Required Field Documentation

Current `protoc-gen-openapiv2` generates OpenAPI schema `required` entries from `google.api.field_behavior = REQUIRED`.

Do not also write `required` inside `openapiv2_field`. Doing both can generate duplicate `required` items.

When a request input field is runtime-required, document it with `openapiv2_field.title` plus `google.api.field_behavior = REQUIRED`. Use this pattern:

```protobuf
// Name 镜像名称
string name = 5 [
  (tagger.tags) = "validate:\"required\" label:\"镜像名称\"",
  (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
    title: "镜像名称"
  },
  (google.api.field_behavior) = REQUIRED
];
```

Rules:

- If `tagger.tags` contains `validate:"required` for an input field, add `openapiv2_field.title` and `google.api.field_behavior = REQUIRED`.
- `openapiv2_field.title` should match the user-facing Chinese `label`.
- Add `openapiv2_field.description` when the comment contains extra business meaning that should be visible in docs.
- This applies to scalar fields and nested request message fields such as `workspace`, `member`, `id`, `name`, and selector fields.
- Entity/data message fields count as input fields when the entity is embedded in create/update request messages. For example, if `Workspace.name` or `Iso.name` has `validate:"required"` because `Workspace`/`Iso` is used by an add/update request, that entity field also needs `openapiv2_field.title` and `field_behavior = REQUIRED`.
- For `omitempty` optional filters, normally add comments and `label` but do not add `field_behavior = REQUIRED`.
- For response-only fields, do not add runtime `validate` tags and do not mark them required just because responses usually contain them.
- Do not write `required: ["field_name"]` or `required: "field_name"` inside `openapiv2_field`.

Example selector field:

```protobuf
// Id 工作空间id
uint64 id = 1 [
  (tagger.tags) = "validate:\"required\" label:\"工作空间\" copier:\"ID\"",
  (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
    title: "工作空间"
  },
  (google.api.field_behavior) = REQUIRED
];
```

Example field with useful doc description:

```protobuf
// Filename 文件上传时的随机时间戳(毫秒)
string filename = 1 [
  (tagger.tags) = "validate:\"required\" label:\"任务名称\"",
  (grpc.gateway.protoc_gen_openapiv2.options.openapiv2_field) = {
    title: "任务名称";
    description: "文件上传时的随机时间戳(毫秒)"
  },
  (google.api.field_behavior) = REQUIRED
];
```

### OpenAPI Checklist

Before leaving a proto file, verify:

- File imports include `google/api/field_behavior.proto` when any field uses `field_behavior`.
- File imports include `protoc-gen-openapiv2/options/annotations.proto`.
- File has complete `openapiv2_swagger`.
- Every service has `openapiv2_tag`.
- Every RPC has `openapiv2_operation`.
- Every RPC auth description matches backend classification from `openPack`, `justLoginPack`, or default protected behavior.
- File-level `security_definitions` defines `BearerAuth`, but file-level global `security` is absent.
- Every non-public RPC has operation-level `BearerAuth` security.
- Public RPCs have no operation-level security.
- Protected RPC descriptions include the exact `<UPPERCASE_FULL_SERVICE>/<UPPERCASE_METHOD>` permission identifier.
- Every runtime-required input field has `tagger.tags validate:"required"`, `label`, `openapiv2_field.title`, and `google.api.field_behavior = REQUIRED`.
- No `openapiv2_field.required` entries are present.
- Optional filters are documented but not marked required.
- Response-only fields are documented but not given input validation.

## gRPC HTTP Compatibility Mapping

This project is not RESTful API design, and it is not "REST but only POST". The interface identity is gRPC `package.Service/Method`; grpc-gateway exposes a compatibility HTTP entry.

Use:

```protobuf
option (google.api.http) = {
  post: "/user.v1.DeptService/AddDept",
  body: "*"
};
```

Rules:

- Use `post` only.
- Use `/<package>.<Service>/<Method>` as the path.
- Treat that path as a method selector, not a resource hierarchy.
- Always set `body: "*"`.
- Put all business inputs in the request message.
- Do not use `get`, `put`, `delete`, query input, or path variables such as `{id}` and `{filename}`.
- Do not infer HTTP verbs from method names like `GetUser`, `UpdateUser`, or `DeleteUser`.
- Comments such as `RequestURL: /<package_name>.<version>.<service_name>/{method}` use `{method}` as a template placeholder, not a business path variable.
- When adapting examples that use `get`, `put`, `delete`, or `/api/v1/.../{id}` paths, keep their OpenAPI documentation annotation shape but convert the HTTP mapping to POST-only `/<package>.<Service>/<Method>` with `body: "*"`.

Why this matters:

- The HTTP server registers generated grpc-gateway handlers.
- POST bodies become protobuf request messages and enter the same controller/middleware path.
- API registration, Casbin API IDs, frontend `api-manifest.ts`, and permission contracts all derive from gRPC service/method metadata, not RESTful routes.

## Comments And Field Design

Write comments before writing fields. Service, rpc, message, and field comments are used by:

- OpenAPI output.
- Generated Go comments.
- Frontend/backend developers.
- Future agents reading the contract.

Comments must explain business meaning, not just repeat the field name.

Field rules:

- Use stable field numbers. Do not reuse numbers casually.
- Use snake_case field names; generated Go and JSON naming handle conversion.
- Protobuf `uint64` and `fixed64` fields serialize to JSON strings. Frontend ID state and payloads must preserve them as strings to avoid JavaScript precision loss.
- Keep request/response names consistent with current domain style, usually `ActionResourceRequest` and `ActionResourceResponse`.
- Keep list response count/total fields aligned with frontend pagination names.

## Validation And Documentation Tags

Distinguish these mechanisms:

- `tagger.tags validate:"required"`: runtime validator v10 input validation after Go struct tag generation.
- `tagger.tags label:"..."`: human-readable field names in validation errors.
- `google.api.field_behavior = REQUIRED`: documentation/client contract, not runtime validation.
- `openapiv2_field.title/description`: OpenAPI documentation, not runtime validation.

If a field is required at runtime, write a `validate` tag. For request input fields, also write `openapiv2_field.title` and `google.api.field_behavior = REQUIRED`. Do not write `openapiv2_field.required` because `field_behavior = REQUIRED` already generates OpenAPI required entries in this project.

## Request And Response Boundary

Put input validation on request messages or input message fields.

Do not put validator v10 input rules on response-only fields. Response comments document returned data; conversion happens through `copier`, field name compatibility, model conversion methods, or explicit controller assembly.

Existing entity messages such as `Dept`, `User`, and `Role` may contain `validate` tags because the same entity is embedded in add/update requests. Treat that as a reuse boundary, not as permission to validate every response field. For new APIs, consider separate input messages when reuse would make response-only fields carry input constraints.

Field classification checklist:

- Create/update input: add `validate` and `label` when runtime validation is needed.
- Path or selector input: still lives in the request body because HTTP is gRPC compatibility POST.
- List filter input: use `omitempty` validation and document special values such as `0` meaning all.
- Pagination input: `page` should be `required,gte=1`; `limit` should be bounded; `order` should be `omitempty,oneof=desc asc`.
- Response-only stored field: comment it, add copier tag if needed, do not add runtime validation.
- Response-only computed field: implement a mysql model `XxxToRPC()` method or explicit controller assembly.
- Response field sourced from an association: add `copier:"XxxToRPC"` and implement the mysql model method, unless the controller assigns it manually for one endpoint.
- Timestamp output: use `google.protobuf.Timestamp` and copier tags pointing to `CreatedAtToRPC` style methods.

## Password And Login Crypto Fields

For login, password change, password confirmation, and similar password entry points, proto request messages must expose encrypted transport fields, not plaintext/MD5 fields:

- Use `password_cipher`, `key_id`, and `challenge_id`.
- Mark them required when that request always needs password confirmation.
- Use `omitempty` only when the password confirmation is conditional, such as phone change requiring current password only when phone is present.
- Reserve removed field numbers and names for old `password`, `old_password`, `oldPassword`, or MD5/hash transport fields.
- Comments must state that `password_cipher` is an RSA-OAEP-SHA256 encrypted JSON payload created from `GetLoginCrypto` output.
- Refresh-token login is the exception: it sends a refresh token and does not require password crypto challenge fields.

Do not add `password`, `old_password`, `new_password`, `password_md5`, `hash`, OPAQUE messages, or stable password-equivalent fields to API request payloads. `newPassword` and `oldPassword` may exist only inside the encrypted JSON payload before frontend encryption.

## `tagger.tags`

`tagger.tags` is not prose. Through `buf.gen.tag.yaml` and `protoc-gen-gotag`, it becomes Go struct tags:

- `validate` for validator v10.
- `label` for readable validation errors.
- `copier` for `copier.Copy` response conversion.

Fields with Go/model naming mismatches need explicit tags, for example `copier:"ID"`, `copier:"ParentID"`, or `copier:"DeptID"`.

Fields sourced from associated models also need explicit tags. For example, a response `username` sourced from `WorkspaceMemberModel.User.Username` should use `copier:"UsernameToRPC"` and a `WorkspaceMemberModel.UsernameToRPC()` method. Do not assume `copier.Copy` reads nested associations automatically.

## `errors.proto`

Use package-level `errors.proto` for reusable business errors:

```protobuf
// @plugins=protoc-gen-go-errors
enum ErrorUser {
  // @code=INTERNAL 内部服务错误
  ERROR_USER_UNKNOWN_UNSPECIFIED = 0;
  // @code=UNKNOWN 组织不允许删除
  ERROR_USER_DEPT_NOT_DEL = 2;
}
```

The generated `errors_errors.pb.go` provides constructors such as `ErrorUserDeptNotDel()`, which services can return and enrich with `.WithMessage(...)`. Do not replace reusable business errors with scattered bare strings.

## Do Not

- Do not hand-edit generated files.
- Do not omit comments because proto lint and docs depend on them.
- Do not copy unrelated business paths such as `/api/v1/vm/...` into CoreAdmin/EgoAdmin rules.
- Do not mix gRPC service/method paths with RESTful path variables.
- Do not write `openapiv2_field.required`; use `google.api.field_behavior = REQUIRED`.
- Do not write runtime `validate` without useful comments and `label`.
- Do not omit file-level `openapiv2_swagger`, service-level `openapiv2_tag`, RPC-level `openapiv2_operation`, operation auth descriptions, or required-field OpenAPI annotations in new proto files.
- Do not add file-level global `security`; it would incorrectly mark public RPCs as requiring Authorization.
- Do not mark optional list filters required through `field_behavior`.
- Do not expose plaintext/MD5 password transport fields in proto requests.

## Validation

- Generate: `make gen`.
- Lint proto: `buf lint`.
- Check forbidden mappings: `rg -n "get:|put:|delete:|\\{id\\}|\\{filename\\}" api/proto`.
- Check generated tags: `rg -n "validate:|label:|copier:" api/gen/go`.
- Check OpenAPI output when docs are affected: `api/openapi/apidocs.swagger.json`.
- Check missing file/service docs in new proto: `rg -n "openapiv2_swagger|openapiv2_tag|openapiv2_operation|openapiv2_field|field_behavior" api/proto`.
- Check duplicate required risk: `rg -n "required:" api/proto`.
- Check OpenAPI auth source before docs: inspect the relevant `internal/app/<service>/server/grpc_server.go`.
- Check protected permission identifiers: confirm descriptions use `<UPPERCASE_FULL_SERVICE>/<UPPERCASE_METHOD>` and match `RegisterAPIs`/Casbin.
- Check password transport fields: `rg -n "password|old_password|password_md5|md5|OPAQUE|passwordauth" api/proto`.
