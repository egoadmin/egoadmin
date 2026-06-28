package egoadmin

import "embed"

// FrontendAssets holds the embedded frontend build output.
//
//go:embed all:web/dist
var FrontendAssets embed.FS

// OpenAPIYAML holds the generated OpenAPI document.
//
//go:embed api/openapi/openapi.yaml
var OpenAPIYAML []byte

// APICatalog holds the generated API catalog.
//
//go:embed api/catalog/api-catalog.json
var APICatalog []byte
