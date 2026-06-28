package main

import "testing"

func TestMergeSpecMergesGenericOpenAPIFragment(t *testing.T) {
	base := map[string]any{
		"paths": map[string]any{},
		"definitions": map[string]any{
			"rpcStatus": map[string]any{"type": "object"},
		},
		"responses": map[string]any{
			"Error": map[string]any{"description": "Error response."},
		},
		"tags": []any{map[string]any{"name": "Existing"}},
	}
	doc := map[string]any{
		"paths": map[string]any{
			"/example": map[string]any{
				"post": map[string]any{
					"description": "extra description",
					"parameters": []any{
						map[string]any{"name": "id", "in": "query"},
					},
					"responses": map[string]any{"200": map[string]any{"description": "OK"}},
				},
			},
		},
		"definitions": map[string]any{
			"exampleResponse": map[string]any{"type": "object"},
		},
		"parameters": map[string]any{
			"TraceID": map[string]any{"name": "X-Trace-ID", "in": "header"},
		},
		"responses": map[string]any{
			"Conflict": map[string]any{"description": "Conflict."},
		},
		"securityDefinitions": map[string]any{
			"ExampleAuth": map[string]any{"type": "apiKey"},
		},
		"tags": []any{
			map[string]any{"name": "Existing"},
			map[string]any{"name": "Example"},
		},
	}

	mergeSpec(base, doc)

	examplePost := base["paths"].(map[string]any)["/example"].(map[string]any)["post"].(map[string]any)
	if examplePost["description"] != "extra description" {
		t.Fatalf("description = %#v, want extra description", examplePost["description"])
	}
	responses := examplePost["responses"].(map[string]any)
	if _, ok := responses["200"]; !ok {
		t.Fatalf("responses = %#v, want 200 response", responses)
	}
	if _, ok := base["definitions"].(map[string]any)["exampleResponse"]; !ok {
		t.Fatalf("definitions missing exampleResponse")
	}
	if _, ok := base["parameters"].(map[string]any)["TraceID"]; !ok {
		t.Fatalf("parameters missing TraceID")
	}
	if _, ok := base["responses"].(map[string]any)["Conflict"]; !ok {
		t.Fatalf("responses missing Conflict")
	}
	if _, ok := base["securityDefinitions"].(map[string]any)["ExampleAuth"]; !ok {
		t.Fatalf("securityDefinitions missing ExampleAuth")
	}
	tags := base["tags"].([]any)
	if len(tags) != 2 {
		t.Fatalf("tags = %#v, want existing plus new tag without duplicate", tags)
	}
}
