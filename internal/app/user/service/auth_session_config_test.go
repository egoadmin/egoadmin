package service

import (
	"testing"
	"time"

	"github.com/egoadmin/egoadmin/internal/component/authsession"
	"github.com/egoadmin/egoadmin/internal/platform/config"
)

func TestAuthSessionRefreshTokenTTL(t *testing.T) {
	tests := []struct {
		name string
		conf config.UserConf
		want time.Duration
	}{
		{
			name: "uses component default when unset",
			conf: config.UserConf{},
			want: authsession.DefaultConfig().RefreshTokenTTL,
		},
		{
			name: "uses configured refresh token expire seconds",
			conf: config.UserConf{RefreshTokenExpire: 3600},
			want: time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authSessionRefreshTokenTTL(tt.conf)
			if got != tt.want {
				t.Fatalf("authSessionRefreshTokenTTL() = %s, want %s", got, tt.want)
			}
		})
	}
}
