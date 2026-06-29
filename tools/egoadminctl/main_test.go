package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyReplacementsLongestFirst(t *testing.T) {
	from := identity{
		Name:      "CoreAdmin",
		Slug:      "coreadmin",
		Module:    "github.com/acme/coreadmin",
		EnvPrefix: "COREADMIN",
		GoPackage: "coreadmin",
	}
	to := identity{
		Name:      "EgoAdmin",
		Slug:      "egoadmin",
		Module:    "github.com/egoadmin/egoadmin",
		EnvPrefix: "EGOADMIN",
		GoPackage: "egoadmin",
	}
	replacements := buildReplacements(from, to, []string{"gateway", "user"}, nil)

	input := []byte(strings.Join([]string{
		`module github.com/acme/coreadmin`,
		`name = "coreadmin-gateway"`,
		`dsn = "coreadmin:coreadmin@tcp(127.0.0.1:3306)/coreadmin_gateway"`,
		`env = "COREADMIN_ATLAS_MIGRATED"`,
	}, "\n"))
	got, count := applyReplacements(input, replacements, nil)
	if count == 0 {
		t.Fatal("expected replacements")
	}
	text := string(got)
	for _, unexpected := range []string{
		"coreadmin",
		"COREADMIN",
		"github.com/acme/coreadmin",
	} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("result still contains %q:\n%s", unexpected, text)
		}
	}
	for _, expected := range []string{
		"github.com/egoadmin/egoadmin",
		"egoadmin-gateway",
		"egoadmin_gateway",
		"EGOADMIN_ATLAS_MIGRATED",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("result missing %q:\n%s", expected, text)
		}
	}
}

func TestExternalDepsProtected(t *testing.T) {
	from := identity{Name: "EgoAdmin", Slug: "egoadmin", Module: "github.com/egoadmin/egoadmin", EnvPrefix: "EGOADMIN", GoPackage: "egoadmin"}
	to := identity{Name: "MyProject", Slug: "myproject", Module: "github.com/acme/myproject", EnvPrefix: "MYPROJECT", GoPackage: "myproject"}
	replacements := buildReplacements(from, to, []string{"gateway", "user"}, nil)

	externals := []string{"github.com/egoadmin/elib", "github.com/egoadmin/eminio", "github.com/egoadmin/xgin"}

	input := []byte(strings.Join([]string{
		`require (`,
		`	github.com/egoadmin/egoadmin v1.0.0`,
		`	github.com/egoadmin/elib v1.0.0`,
		`	github.com/egoadmin/eminio v1.0.0`,
		`	github.com/egoadmin/xgin v1.0.0`,
		`)`,
		`import "github.com/egoadmin/elib/pkg/util/xorm"`,
		`name = "egoadmin-gateway"`,
	}, "\n"))

	got, _ := applyReplacements(input, replacements, externals)
	text := string(got)

	// Main module should be renamed
	if !strings.Contains(text, "github.com/acme/myproject") {
		t.Fatal("main module should be renamed")
	}
	// External deps must NOT be renamed
	for _, ext := range externals {
		if !strings.Contains(text, ext) {
			t.Fatalf("external dep %q should be preserved, got:\n%s", ext, text)
		}
	}
	// elib subpath must be preserved
	if !strings.Contains(text, "github.com/egoadmin/elib/pkg/util/xorm") {
		t.Fatal("elib subpath should be preserved")
	}
	// Service name should be renamed
	if strings.Contains(text, "egoadmin-gateway") {
		t.Fatal("service name should be renamed")
	}
	if !strings.Contains(text, "myproject-gateway") {
		t.Fatal("service name should use new slug")
	}
}

func TestRenameProjectDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module github.com/acme/coreadmin\n")
	writeFile(t, root, ".egoadmin/template.json", `{"name":"CoreAdmin","slug":"coreadmin","module":"github.com/acme/coreadmin","envPrefix":"COREADMIN","goPackage":"coreadmin","services":["gateway","user"]}`+"\n")
	writeFile(t, root, "api/gen/go/user/v1/user.pb.go", "github.com/acme/coreadmin\n")
	writeFile(t, root, ".agents/skills/doc.md", "CoreAdmin coreadmin\n")
	writeFile(t, root, ".claude/skills/doc.md", "CoreAdmin coreadmin\n")

	changes, err := renameProject(renameOptions{
		Root: root,
		From: identity{
			Name:      "CoreAdmin",
			Slug:      "coreadmin",
			Module:    "github.com/acme/coreadmin",
			EnvPrefix: "COREADMIN",
			GoPackage: "coreadmin",
		},
		To: identity{
			Name:      "EgoAdmin",
			Slug:      "egoadmin",
			Module:    "github.com/egoadmin/egoadmin",
			EnvPrefix: "EGOADMIN",
			GoPackage: "egoadmin",
		},
		Services:     []string{"gateway", "user"},
		Write:        false,
		UpdateConfig: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("changes = %#v, want go.mod and config", changes)
	}
	data := readFile(t, root, "go.mod")
	if strings.Contains(data, "egoadmin") {
		t.Fatalf("dry-run wrote go.mod: %s", data)
	}
	generated := readFile(t, root, "api/gen/go/user/v1/user.pb.go")
	if !strings.Contains(generated, "coreadmin") {
		t.Fatalf("generated file should be skipped: %s", generated)
	}
	agent := readFile(t, root, ".agents/skills/doc.md")
	if !strings.Contains(agent, "CoreAdmin") {
		t.Fatalf(".agents should be skipped by default: %s", agent)
	}
	claude := readFile(t, root, ".claude/skills/doc.md")
	if !strings.Contains(claude, "CoreAdmin") {
		t.Fatalf(".claude should be skipped by default: %s", claude)
	}
}

func TestRenameProjectWriteUpdatesConfig(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".egoadmin/template.json", `{
  "name": "CoreAdmin",
  "slug": "coreadmin",
  "module": "github.com/egoadmin/egoadmin",
  "envPrefix": "COREADMIN",
  "goPackage": "coreadmin",
  "services": ["gateway", "user"]
}
`)
	writeFile(t, root, "embed.go", "package coreadmin\n")

	changes, err := renameProject(renameOptions{
		Root: root,
		To: identity{
			Name:      "EgoAdmin",
			Slug:      "egoadmin",
			Module:    "github.com/egoadmin/egoadmin",
			EnvPrefix: "EGOADMIN",
			GoPackage: "egoadmin",
		},
		Write:        true,
		UpdateConfig: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) == 0 {
		t.Fatal("expected changes")
	}
	if got := readFile(t, root, "embed.go"); got != "package egoadmin\n" {
		t.Fatalf("embed.go = %q", got)
	}
	cfg := readFile(t, root, ".egoadmin/template.json")
	if !strings.Contains(cfg, `"slug": "egoadmin"`) {
		t.Fatalf("config not updated:\n%s", cfg)
	}
}

func TestRunInitDryRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"init",
		"--dry-run",
		"--repo", "https://example.com/template.git",
		"--dest", "/tmp/example",
		"--name", "DemoAdmin",
		"--slug", "demoadmin",
		"--module", "github.com/acme/demoadmin",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "dry-run: git clone") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunInitDryRunUsesDefaultRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"init",
		"--dry-run",
		"--dest", "/tmp/example",
		"--name", "DemoAdmin",
		"--slug", "demoadmin",
		"--module", "github.com/acme/demoadmin",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), defaultTemplateRepo) {
		t.Fatalf("stdout = %q, want default repo %q", stdout.String(), defaultTemplateRepo)
	}
}

func TestSanitizeGoPackage(t *testing.T) {
	tests := []struct {
		name string
		slug string
		want string
	}{
		{name: "plain", slug: "egoadmin", want: "egoadmin"},
		{name: "hyphen", slug: "ego-admin", want: "egoadmin"},
		{name: "digit prefix", slug: "123-admin", want: "app123admin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeGoPackage(tt.slug); got != tt.want {
				t.Fatalf("sanitizeGoPackage(%q) = %q, want %q", tt.slug, got, tt.want)
			}
		})
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
