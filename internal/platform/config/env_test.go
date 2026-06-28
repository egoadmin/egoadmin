package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnvSuffix(t *testing.T) {
	got := EnvSuffix("component", "idgen", "codec", "secret")
	if got != "COMPONENT_IDGEN_CODEC_SECRET" {
		t.Fatalf("EnvSuffix() = %q, want COMPONENT_IDGEN_CODEC_SECRET", got)
	}

	got = EnvSuffix("client", "minio", "accessKeyID")
	if got != "CLIENT_MINIO_ACCESSKEYID" {
		t.Fatalf("EnvSuffix() = %q, want CLIENT_MINIO_ACCESSKEYID", got)
	}
}

func TestApplyEnvOverridesUsesMergedConfigTypes(t *testing.T) {
	mg := &Manager{envPrefix: "TESTAPP"}
	config := map[string]interface{}{
		"app": map[string]interface{}{
			"service": map[string]interface{}{
				"name":        "egoadmin",
				"autoMigrate": false,
				"retry":       1,
				"workerCount": int64(2),
				"maxOpen":     uint(10),
				"ratio":       0.2,
				"ports":       []interface{}{"9001"},
				"headers":     []string{"Content-Type"},
			},
		},
		"component": map[string]interface{}{
			"task": map[string]interface{}{
				"timeout": 5 * time.Second,
			},
		},
	}

	t.Setenv("TESTAPP_APP_SERVICE_NAME", "demo")
	t.Setenv("TESTAPP_APP_SERVICE_AUTOMIGRATE", "true")
	t.Setenv("TESTAPP_APP_SERVICE_RETRY", "3")
	t.Setenv("TESTAPP_APP_SERVICE_WORKERCOUNT", "8")
	t.Setenv("TESTAPP_APP_SERVICE_MAXOPEN", "32")
	t.Setenv("TESTAPP_APP_SERVICE_RATIO", "0.75")
	t.Setenv("TESTAPP_APP_SERVICE_PORTS", "9001, 9002")
	t.Setenv("TESTAPP_APP_SERVICE_HEADERS", "Content-Type, Authorization")
	t.Setenv("TESTAPP_COMPONENT_TASK_TIMEOUT", "12s")
	t.Setenv("TESTAPP_APP_SERVICE_UNKNOWN", "ignored")

	if err := mg.applyEnvOverrides(config); err != nil {
		t.Fatal(err)
	}

	service := config["app"].(map[string]interface{})["service"].(map[string]interface{})
	if service["name"] != "demo" {
		t.Fatalf("name = %v, want demo", service["name"])
	}
	if service["autoMigrate"] != true {
		t.Fatalf("autoMigrate = %v, want true", service["autoMigrate"])
	}
	if service["retry"] != 3 {
		t.Fatalf("retry = %#v, want int(3)", service["retry"])
	}
	if service["workerCount"] != int64(8) {
		t.Fatalf("workerCount = %#v, want int64(8)", service["workerCount"])
	}
	if service["maxOpen"] != uint(32) {
		t.Fatalf("maxOpen = %#v, want uint(32)", service["maxOpen"])
	}
	if service["ratio"] != 0.75 {
		t.Fatalf("ratio = %#v, want 0.75", service["ratio"])
	}
	ports := service["ports"].([]interface{})
	if len(ports) != 2 || ports[0] != "9001" || ports[1] != "9002" {
		t.Fatalf("ports = %#v, want [9001 9002]", ports)
	}
	headers := service["headers"].([]string)
	if len(headers) != 2 || headers[0] != "Content-Type" || headers[1] != "Authorization" {
		t.Fatalf("headers = %#v, want [Content-Type Authorization]", headers)
	}

	task := config["component"].(map[string]interface{})["task"].(map[string]interface{})
	if task["timeout"] != 12*time.Second {
		t.Fatalf("timeout = %#v, want 12s", task["timeout"])
	}
	if _, ok := service["unknown"]; ok {
		t.Fatal("unknown env created hidden config key")
	}
}

func TestApplyEnvOverridesReturnsConversionError(t *testing.T) {
	mg := &Manager{envPrefix: "TESTAPP"}
	config := map[string]interface{}{
		"app": map[string]interface{}{
			"service": map[string]interface{}{
				"autoMigrate": false,
			},
		},
	}
	t.Setenv("TESTAPP_APP_SERVICE_AUTOMIGRATE", "not-bool")

	err := mg.applyEnvOverrides(config)
	if err == nil {
		t.Fatal("expected conversion error")
	}
	if !strings.Contains(err.Error(), "TESTAPP_APP_SERVICE_AUTOMIGRATE") {
		t.Fatalf("error = %v, want env name", err)
	}
}

func TestBuildEnvBindingsDetectsSuffixConflict(t *testing.T) {
	config := map[string]interface{}{
		"app": map[string]interface{}{
			"db": map[string]interface{}{
				"url": "a",
			},
			"DB": map[string]interface{}{
				"url": "b",
			},
		},
	}

	_, err := buildEnvBindings(config)
	if err == nil {
		t.Fatal("expected suffix conflict")
	}
	if !strings.Contains(err.Error(), "APP_DB_URL") {
		t.Fatalf("error = %v, want APP_DB_URL", err)
	}
}

func TestLoadDotEnvDoesNotOverrideSystemEnv(t *testing.T) {
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err = os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if er := os.Chdir(oldwd); er != nil {
			t.Fatalf("restore working directory: %v", er)
		}
	})

	envPath := filepath.Join(tmp, ".env")
	if err = os.WriteFile(envPath, []byte("TESTAPP_APP_NAME=from-dotenv\nTESTAPP_APP_KEEP=from-dotenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TESTAPP_APP_NAME", "from-system")

	mg := &Manager{envPrefix: "TESTAPP"}
	if err = mg.loadDotEnv(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TESTAPP_APP_NAME"); got != "from-system" {
		t.Fatalf("system env overwritten by .env: %q", got)
	}
	if got := os.Getenv("TESTAPP_APP_KEEP"); got != "from-dotenv" {
		t.Fatalf("dotenv value = %q, want from-dotenv", got)
	}
}
