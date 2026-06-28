package config

import (
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

func TestDefaultConfigDocumentIsServiceScoped(t *testing.T) {
	t.Parallel()

	gatewayDoc := DefaultConfigDocument("TESTAPP", ServiceGateway)
	if !strings.Contains(gatewayDoc, "TESTAPP_<EnvSuffix>") {
		t.Fatalf("gateway document missing env prefix header")
	}
	if !strings.Contains(gatewayDoc, "file://atlas/migrations/gateway") {
		t.Fatalf("gateway document missing gateway migration dir")
	}
	if strings.Contains(gatewayDoc, "egoadmin_user") || strings.Contains(gatewayDoc, "127.0.0.1:6380") {
		t.Fatalf("gateway document contains user-only runtime defaults")
	}

	userDoc := DefaultConfigDocument("TESTAPP", ServiceUser)
	if !strings.Contains(userDoc, "file://atlas/migrations/user") {
		t.Fatalf("user document missing user migration dir")
	}
	if strings.Contains(userDoc, "egoadmin_gateway") || strings.Contains(userDoc, "127.0.0.1:6379") {
		t.Fatalf("user document contains gateway-only runtime defaults")
	}
}

func TestDefaultConfigDocumentIncludesEnvSuffixGuidance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		service Service
		want    []string
	}{
		{
			name:    "gateway",
			service: ServiceGateway,
			want: []string{
				"EnvSuffix: APP_SERVICE_NAME",
				"EnvSuffix: APP_SHUTDOWN_STOPTIMEOUT",
				"EnvSuffix: APP_WEB_OFFLINEONPAGELEAVE",
				"EnvSuffix: APP_DBMIGRATION_URL",
				"EnvSuffix: CLIENT_GRPC_USER_ADDR",
				"EnvSuffix: CLIENT_MYSQL_DSN",
				"EnvSuffix: CLIENT_REDIS_PASSWORD",
				"EnvSuffix: CLIENT_MINIO_SECRETACCESSKEY",
				"EnvSuffix: ETCD_ADDRS",
				"EnvSuffix: REGISTRY_SCHEME",
				"EnvSuffix: TRACE_SERVICENAME",
				"EnvSuffix: TRACE_OTLP_ENDPOINT",
				"EnvSuffix: COMPONENT_IDGEN_CODEC_SECRET",
				"EnvSuffix: COMPONENT_UPLOAD_TUS_TEMPORARYDIRECTORY",
				"EnvSuffix: COMPONENT_CDN_SIGNSECRET",
				"EnvSuffix: CLIENT_IMAGEPROCESSOR_URL",
				"EnvSuffix: CLIENT_IMAGEPROCESSOR_SECRET",
				"EnvSuffix: SERVER_HTTP_ACCESSCONTROLEXPOSEHEADERS",
			},
		},
		{
			name:    "user",
			service: ServiceUser,
			want: []string{
				"EnvSuffix: APP_SERVICE_NAME",
				"EnvSuffix: APP_SHUTDOWN_STOPTIMEOUT",
				"EnvSuffix: APP_DBMIGRATION_URL",
				"EnvSuffix: APP_USER_ADMINPASSWORD",
				"EnvSuffix: APP_USER_JWTSIGNKEY",
				"EnvSuffix: APP_USER_HEARTBEATOFFLINESECONDS",
				"EnvSuffix: APP_USER_REVOKESESSIONONHEARTBEATOFFLINE",
				"EnvSuffix: CLIENT_MYSQL_DSN",
				"EnvSuffix: CLIENT_REDIS_PASSWORD",
				"EnvSuffix: ETCD_ADDRS",
				"EnvSuffix: REGISTRY_SCHEME",
				"EnvSuffix: TRACE_SERVICENAME",
				"EnvSuffix: TRACE_OTLP_ENDPOINT",
				"EnvSuffix: COMPONENT_IDGEN_CODEC_SECRET",
				"EnvSuffix: CRON_USER_LOGIN_OFFLINE_SPEC",
			},
		},
		{
			name:    "idgen",
			service: ServiceIDGen,
			want: []string{
				"EnvSuffix: APP_SERVICE_NAME",
				"EnvSuffix: APP_IDGEN_MACHINELEASECLEANUPRETENTION",
				"EnvSuffix: APP_IDGEN_MACHINELEASECLEANUPLIMIT",
				"EnvSuffix: APP_SHUTDOWN_STOPTIMEOUT",
				"EnvSuffix: APP_DBMIGRATION_URL",
				"EnvSuffix: CLIENT_MYSQL_DSN",
				"EnvSuffix: CRON_IDGEN_MACHINE_CLEANUP_SPEC",
				"EnvSuffix: ETCD_ADDRS",
				"EnvSuffix: REGISTRY_SCHEME",
				"EnvSuffix: TRACE_SERVICENAME",
				"EnvSuffix: TRACE_OTLP_ENDPOINT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc := DefaultConfigDocument("TESTAPP", tt.service)
			for _, want := range tt.want {
				if !strings.Contains(doc, want) {
					t.Fatalf("default config document missing %q", want)
				}
			}
		})
	}
}

func TestDefaultGatewayUserClientReadTimeoutAllowsColdStart(t *testing.T) {
	t.Parallel()

	var raw struct {
		Client struct {
			GRPC struct {
				User struct {
					ReadTimeout time.Duration
				}
			}
		}
	}
	if _, err := toml.Decode(defaultGatewayConfigContent, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.Client.GRPC.User.ReadTimeout < 3*time.Second {
		t.Fatalf("gateway user readTimeout = %s, want at least 3s", raw.Client.GRPC.User.ReadTimeout)
	}
}

func TestDefaultIDGenMachineLeaseCleanupRetention(t *testing.T) {
	t.Parallel()

	var raw struct {
		App struct {
			IDGen struct {
				MachineLeaseCleanupRetention time.Duration
				MachineLeaseCleanupLimit     int
			}
		}
	}
	if _, err := toml.Decode(defaultIDGenConfigContent, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.App.IDGen.MachineLeaseCleanupRetention != 7*24*time.Hour {
		t.Fatalf("machine lease cleanup retention = %s, want 168h", raw.App.IDGen.MachineLeaseCleanupRetention)
	}
	if raw.App.IDGen.MachineLeaseCleanupLimit != 1000 {
		t.Fatalf("machine lease cleanup limit = %d, want 1000", raw.App.IDGen.MachineLeaseCleanupLimit)
	}
}

func TestDefaultCommonConfigIncludesMachineLeaseTuning(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"EnvSuffix: COMPONENT_IDGEN_MACHINE_TTL",
		"EnvSuffix: COMPONENT_IDGEN_MACHINE_RENEWINTERVAL",
		"EnvSuffix: COMPONENT_IDGEN_MACHINE_RENEWTIMEOUT",
		"EnvSuffix: COMPONENT_IDGEN_MACHINE_MINRENEWWINDOWS",
		"EnvSuffix: COMPONENT_IDGEN_MACHINE_REALLOCATEBACKOFF",
		"ttl = \"60s\"",
		"renewInterval = \"10s\"",
		"renewTimeout = \"5s\"",
		"minRenewWindows = 5",
		"reallocateBackoff = \"2s\"",
	} {
		if !strings.Contains(defaultCommonConfigContent, want) {
			t.Fatalf("default common config missing machine lease tuning %q", want)
		}
	}
}

func TestDefaultGatewayCorsAllowsTusBrowserUpload(t *testing.T) {
	t.Parallel()

	var raw struct {
		Server struct {
			HTTP struct {
				AccessControlAllowHeaders  []string
				AccessControlAllowMethods  []string
				AccessControlExposeHeaders []string
			}
		}
	}
	if _, err := toml.Decode(defaultGatewayConfigContent, &raw); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"Tus-Resumable",
		"Upload-Length",
		"Upload-Offset",
		"Upload-Metadata",
		"Upload-Defer-Length",
		"Upload-Concat",
	} {
		if !containsString(raw.Server.HTTP.AccessControlAllowHeaders, want) {
			t.Fatalf("gateway CORS allow headers missing %q", want)
		}
	}

	for _, want := range []string{"PATCH", "HEAD", "OPTIONS"} {
		if !containsString(raw.Server.HTTP.AccessControlAllowMethods, want) {
			t.Fatalf("gateway CORS allow methods missing %q", want)
		}
	}

	for _, want := range []string{
		"Location",
		"Upload-Offset",
		"Upload-Length",
		"Tus-Resumable",
		"X-Upload-Reference-Id",
		"X-Upload-File-Id",
	} {
		if !containsString(raw.Server.HTTP.AccessControlExposeHeaders, want) {
			t.Fatalf("gateway CORS expose headers missing %q", want)
		}
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
