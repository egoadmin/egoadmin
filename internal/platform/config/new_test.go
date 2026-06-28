package config

import "testing"

// 无 --config 时源文件不存在，New 退化为只用内置默认配置。
// 验证 user 服务的默认配置被正确绑定到 typed *Config。
func TestNewWithServiceUserUsesDefaults(t *testing.T) {
	conf := New(WithService(ServiceUser), WithEnvPrefix(""))
	t.Cleanup(func() { _ = conf.Close() })

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
