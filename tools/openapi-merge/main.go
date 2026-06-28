package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	baseYAML := flag.String("base-yaml", "api/openapi/openapi.yaml", "base OpenAPI YAML")
	baseJSON := flag.String("base-json", "api/openapi/openapi.json", "base OpenAPI JSON")
	extraYAML := flag.String("extra-yaml", "api/httpdoc/openapi.yaml", "additional OpenAPI YAML fragment")
	outYAML := flag.String("out-yaml", "api/openapi/openapi.yaml", "merged OpenAPI YAML output")
	outJSON := flag.String("out-json", "api/openapi/openapi.json", "merged OpenAPI JSON output")
	flag.Parse()

	if err := mergeYAML(*baseYAML, *extraYAML, *outYAML); err != nil {
		fail(err)
	}
	if err := mergeJSON(*baseJSON, *extraYAML, *outJSON); err != nil {
		fail(err)
	}
}

func fail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "openapi merge: %v\n", err)
	os.Exit(1)
}

func mergeYAML(basePath, extraPath, outPath string) error {
	var base map[string]any
	if err := readYAML(basePath, &base); err != nil {
		return err
	}
	if err := mergeExtra(&base, extraPath); err != nil {
		return err
	}
	data, err := yaml.Marshal(base)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	return os.WriteFile(outPath, data, 0o644)
}

func mergeJSON(basePath, extraPath, outPath string) error {
	var base map[string]any
	if err := readJSON(basePath, &base); err != nil {
		return err
	}
	if err := mergeExtra(&base, extraPath); err != nil {
		return err
	}
	data, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(outPath, data, 0o644)
}

func mergeExtra(base *map[string]any, extraPath string) error {
	var extra map[string]any
	if err := readYAML(extraPath, &extra); err != nil {
		return err
	}
	mergeSpec(*base, extra)
	return nil
}

func readYAML(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err = yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return nil
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err = json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return nil
}

func mergeSpec(base, doc map[string]any) {
	deepMergeKey(base, doc, "paths")
	deepMergeKey(base, doc, "definitions")
	deepMergeKey(base, doc, "parameters")
	deepMergeKey(base, doc, "responses")
	deepMergeKey(base, doc, "securityDefinitions")
	mergeTags(base, doc)
}

func deepMergeKey(base, doc map[string]any, key string) {
	src, _ := doc[key].(map[string]any)
	if len(src) == 0 {
		return
	}
	dst, _ := base[key].(map[string]any)
	if dst == nil {
		dst = map[string]any{}
		base[key] = dst
	}
	deepMergeMap(dst, src)
}

func deepMergeMap(dst, src map[string]any) {
	for key, srcValue := range src {
		srcMap, srcIsMap := srcValue.(map[string]any)
		dstMap, dstIsMap := dst[key].(map[string]any)
		if srcIsMap && dstIsMap {
			deepMergeMap(dstMap, srcMap)
			continue
		}
		dst[key] = srcValue
	}
}

func mergeTags(base, doc map[string]any) {
	dst, _ := base["tags"].([]any)
	seen := map[string]struct{}{}
	for _, item := range dst {
		if tag, ok := item.(map[string]any); ok {
			if name, _ := tag["name"].(string); name != "" {
				seen[name] = struct{}{}
			}
		}
	}
	for _, item := range asSlice(doc["tags"]) {
		tag, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tag["name"].(string)
		if name == "" {
			continue
		}
		if _, ok = seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		dst = append(dst, tag)
	}
	base["tags"] = dst
}

func asSlice(value any) []any {
	items, _ := value.([]any)
	if items == nil {
		return []any{}
	}
	return items
}
