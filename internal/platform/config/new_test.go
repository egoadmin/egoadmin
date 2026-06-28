package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/gotomicro/ego/core/econf"
)

func TestNewWithServiceUserDoesNotRequireWebConfig(t *testing.T) {
	if err := econf.LoadFromReader(strings.NewReader(""), toml.Unmarshal); err != nil {
		t.Fatalf("reset econf before test: %v", err)
	}
	t.Cleanup(func() {
		if err := econf.LoadFromReader(strings.NewReader(""), toml.Unmarshal); err != nil {
			t.Fatalf("reset econf: %v", err)
		}
	})

	conf := New(WithService(ServiceUser), WithEnvPrefix(""))
	if conf.App().Name != "egoadmin-user" {
		t.Fatalf("service name = %q, want egoadmin-user", conf.App().Name)
	}
	if conf.DBMigration().Dir != "file://atlas/migrations/user" {
		t.Fatalf("migration dir = %q, want user dir", conf.DBMigration().Dir)
	}
	if conf.User().JwtExpire == 0 {
		t.Fatalf("user config was not loaded")
	}
	if !conf.User().HeartbeatOfflineEnabled {
		t.Fatalf("heartbeat offline should be enabled by default")
	}
	if conf.User().HeartbeatOfflineSeconds != 660 {
		t.Fatalf("heartbeatOfflineSeconds = %d, want 660", conf.User().HeartbeatOfflineSeconds)
	}
	if conf.User().RevokeSessionOnHeartbeatOffline {
		t.Fatalf("revokeSessionOnHeartbeatOffline should be disabled by default")
	}
}
