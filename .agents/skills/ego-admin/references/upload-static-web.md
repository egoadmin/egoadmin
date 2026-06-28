# Upload And Static Web

Read this before changing gateway upload behavior, upload lifecycle, TUS upload, MinIO/S3 behavior, embedded `web/dist`, SPA fallback, runtime frontend config, or static asset serving.

## Evidence Paths

- `internal/app/gateway/server/**`
- `internal/app/gateway/adapter/persistence/mysql/upload_*`
- `internal/app/gateway/internal/job/**`
- `internal/app/gateway/internal/web/**`
- `internal/component/upload/**`
- `internal/component/cdn/**`
- `internal/component/etusupload/**` for legacy reference only
- `internal/platform/objectstore/**`
- `api/httpdoc/openapi.yaml`
- `tools/openapi-merge/**`
- `atlas/migrations/gateway/**`
- `web/src/api/modules/upload.ts`
- `web/src/components/*Upload/**`
- `web/**`

## Ownership

Gateway owns external upload endpoints and embedded web serving. `internal/component/upload` owns reusable upload lifecycle logic and must not import gateway packages.

Rules:

- Do not treat upload as a normal proto API when it intentionally streams multipart/TUS data.
- Keep upload metadata in gateway's database boundary: `file_object`, `file_reference`, and `upload_session`.
- Use logical upload mode: write objects directly under `files/...`; do not implement `tmp -> files` promotion with `CopyObject + DeleteObject`.
- Business APIs should accept `referenceId` and bind the reference after business save succeeds. Keep raw `filepath/objectKey` only as compatibility.
- Download and image access should go through `/cdn/file/{referenceId}` and `/cdn/image/{referenceId}/...`, not public object-store URLs.
- Do not edit `web/dist` by hand as a lasting fix.
- Inject runtime frontend config without secrets.
- Keep SPA fallback from swallowing `/api` errors.
- Keep file/object storage config service-specific and secret-safe.

## File Lifecycle

Use these states deliberately:

- `file_object`: `uploading`, `available`, `deleting`, `deleted`.
- `file_reference`: `temporary`, `bound`, `released`, `expired`.
- `upload_session`: `uploading`, `finished`, `metadata_cleaning`, `metadata_cleaned`, `failed`, `aborted`.

Cleanup rules:

- Expire temporary references only after `expires_at`.
- Never delete an object while it has `bound` references or unexpired `temporary` references.
- Mark objects `deleting` before physical `DeleteObject`.
- Treat object-store `NoSuchKey` as a successful cleanup for idempotency.
- Release old references when a business field is replaced or deleted.

## CDN Download And Image Access

Gateway owns `/cdn` access routes. Do not expose underlying image processor routes or service names in public API paths or OpenAPI docs.

Rules:

- `/cdn/file/{referenceId}` supports Bearer-auth owner checks and short-lived signed URLs.
- `/cdn/file/{referenceId}` should stream from object storage with `http.ServeContent` semantics so Range and inline display work.
- `/cdn/image/{referenceId}` and `/cdn/image/{referenceId}/{processPath...}` use `referenceId`, never raw `objectKey`, for access decisions.
- Image `<img src>` access may be public for bound references by config or signed by `expires/token`; do not rely on `Authorization` for normal `<img>` rendering.
- Temporary references should require a valid signature unless a config explicitly allows temporary image access.
- Released, expired, deleted, or unavailable objects must not be served.
- Only forward safe request and response headers through image proxying.

Implementation rules:

- Keep CDN code in `internal/component/cdn`; do not add ad hoc download handlers in gateway packages.
- Register only `/cdn/file/:referenceId`, `/cdn/image/:referenceId`, and `/cdn/image/:referenceId/*processPath` as public routes.
- Use `upload.GetDownloadReference` and `upload.GetDownloadReferenceForOwner` to resolve metadata; never trust a client-provided object path.
- Use `upload.ObjectStore.Get` for file download. MinIO object readers support `Read`, `Seek`, `Stat`, and `Close`, which are required for `http.ServeContent`.
- Include `display` in signed file URL material when it is present, so `attachment` and `inline` cannot be swapped after signing.
- For image signed URLs, sign the external CDN path and process path. Reject invalid or expired tokens even if `publicImage=true`; a bad token must not be silently ignored as public access.
- Validate image `processPath`: reject empty segments, `.`, `..`, control characters, overlong paths, and overlong query strings.
- Proxy image requests with a bounded `http.Client` timeout. Forward only safe request headers such as `Accept`, `Accept-Encoding`, `If-None-Match`, `If-Modified-Since`, `Range`, and `User-Agent`.
- Preserve only safe image response headers: `Content-Type`, `Content-Length`, `Cache-Control`, `ETag`, `Last-Modified`, `Expires`, `Vary`, `Accept-Ranges`, `Content-Range`, and `X-Content-Type-Options`.
- Map released/expired references and unavailable objects to `410 Gone`; missing references or objects to `404`; bad/expired signatures to `403`; image processor upstream failures to `502/504` as appropriate.
- Keep image processor config names generic: `[client.imageProcessor]`, `EGOADMIN_CLIENT_IMAGEPROCESSOR_*`, and Compose service `image-processor`. Do not expose implementation-specific names in public routes or OpenAPI docs.

Image processor deployment rules:

- Include the default image processor service in Compose when CDN image processing is enabled.
- Keep both `deploy/docker-compose.yml` and local `test/docker-compose.yml` including the image processor service; `make dev-up` must start it for local CDN image testing.
- Pin the image version; do not use `latest`.
- Configure the image processor to read from the same object bucket as uploads.
- Gateway processes running on the host during e2e should use the host-mapped image processor URL; containerized gateway deployments should use the Compose service URL.

## Upload Profiles

Profiles are upload strategies, not per-business-field models. Keep names few and stable, such as `default`, `image`, `avatar`, `document`, and `video`.

Rules:

- Frontend passes a stable `profile`; backend validates profile again.
- `/upload/profiles` is for UI hints only, not authorization.
- Profile-specific security checks happen in the upload component and again when binding references to business fields.
- `tusRequired` profiles must reject multipart uploads and use TUS.

## TUS Upload

Current target component is `internal/component/upload` using tusd `s3store` with AWS SDK v2. `internal/component/etusupload` is legacy/reference material and should not receive new lifecycle work unless explicitly requested.

Rules:

- Use tusd `s3store`; do not hand-roll resumable uploads with `io.Pipe + PutObject`.
- `ObjectPrefix` should be `files`; upload IDs passed to tusd should be relative object IDs.
- `MetadataObjectPrefix` should be `tus-meta`.
- Record the full TUS ID from created/completed events because `s3store` changes IDs to `objectId+multipartId`.
- Completed uploads become `file_object.available` and `upload_session.finished`.
- Clean `.info/.part` metadata asynchronously by exact DB keys only. Validate keys are under the configured metadata prefix.
- Expired unfinished uploads should call tusd store termination when the full TUS ID is known, then mark session `aborted`. If the full ID is not available, mark the DB lifecycle and rely on S3/MinIO multipart lifecycle rules as a safety net.
- Limit and clean the local tusd temporary directory; only delete `tusd-s3-tmp-*` files older than the configured TTL.

## Frontend Upload

Use `web/src/api/modules/upload.ts` as the frontend upload entry.

Rules:

- Keep `upload(formData)` only for compatibility.
- New code should use `uploadFile({ file, profile, sha256, preferTus, onProgress })`.
- Upload components should return and propagate `referenceId`.
- Preserve protobuf `uint64` IDs as strings in TypeScript.
- Frontend may run instant upload precheck, but backend remains the source of truth.

## Permission And Auth

Upload/static web changes can affect auth and frontend runtime behavior. Confirm token validity, refresh behavior, public/static paths, `/api` exclusion, CORS, and object storage URLs.

Upload endpoints are not proto APIs but still require auth unless explicitly designed as public. Health/static paths must remain separate from upload auth.

## OpenAPI Docs

Upload, TUS, file download, and image processing are HTTP-only stream/protocol endpoints. Document them with hand-written Swagger 2.0 YAML in `api/httpdoc/openapi.yaml`.

Rules:

- Do not create doc-only proto services for upload/download/CDN endpoints.
- Do not generate Go, gRPC, gateway, API catalog, frontend API manifest, or permission contracts from HTTP-only docs.
- `tools/openapi-merge` must stay generic: merge `paths`, `definitions`, `parameters`, `responses`, `securityDefinitions`, and tags by name. Do not hardcode `/upload`, `/tus`, `/cdn`, multipart fields, or response headers in Go.
- Keep `api/httpdoc/openapi.yaml` synchronized with real routes and headers.
- Document upload, TUS, file download, and image processing as complete user-facing HTTP contracts: include authentication, request fields, response fields, lifecycle notes, examples, response headers, error codes, and frontend usage notes.
- Keep HTTP-only docs on real routes only. Do not document `/download`, `/imagor`, or other compatibility/example paths as public EgoAdmin APIs.
- Run `make gen` after OpenAPI doc changes so base proto OpenAPI is regenerated, normalized, and then merged with the hand-written HTTP docs.
- Validate generated `/openapi.yaml` includes `/upload`, `/tus/upload`, `/cdn/file/{referenceId}`, and `/cdn/image/{referenceId}` when these routes are changed.

Documentation source rules:

- Put business documentation text in `api/httpdoc/openapi.yaml`, not in `tools/openapi-merge`.
- `tools/openapi-merge` must not know about upload, TUS, CDN, multipart fields, or response headers.
- Do not add doc-only proto services for HTTP-only streaming/protocol endpoints.
- Generated `api/openapi/**` is ignored; validate it after `make gen` but do not commit it unless repository policy changes.

## e2e Requirement

Gateway upload/static web changes normally need gateway e2e or an explicit reason they do not. Cover SPA fallback, `/api` exclusion, runtime config, upload/auth behavior, reference binding, and cleanup behavior as applicable.

CDN image processing e2e must include at least one real image-processor + MinIO path. Unit tests may use `httptest` to verify gateway proxy signing/header behavior, but complete gateway-facing validation must prove an uploaded real image can be processed through `/cdn/image/{referenceId}/...`.

Minimum CDN e2e coverage:

- Upload a document profile via multipart, then download it through `/cdn/file/{referenceId}` with Bearer auth.
- Assert `Content-Disposition` for default attachment and `display=inline`.
- Assert `Range` requests return `206 Partial Content`.
- Assert a missing `referenceId` returns `404`.
- Upload a real image, bind its `referenceId` through a business API, then request `/cdn/image/{referenceId}/...` through the real image processor and MinIO path.
- For image processing, assert an actual processed image response, not only a fake proxy body; for WebP conversion, check both `Content-Type` and a `RIFF/WEBP` body header.
- Assert invalid or expired image signatures return `403`.

## Validation

- `go test -race ./internal/component/upload/...`
- `go test -race ./internal/component/cdn/...`
- `go test -race ./internal/app/gateway/adapter/persistence/mysql ./internal/app/gateway/controller ./internal/app/gateway/server`
- `go test -race ./internal/platform/objectstore/...`
- `cd web && pnpm run type-check`
- `cd web && pnpm run build`
- `make service.check SERVICE=gateway`
- `make migrate.validate SERVICE=gateway` when upload schema changes
- `make e2e E2E_TIMEOUT=20m` for complete gateway-visible upload workflows
