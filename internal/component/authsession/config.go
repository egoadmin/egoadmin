package authsession

import "time"

const (
	// PackageName identifies the component in EGO logs.
	PackageName = "component.authsession"
)

type SameDeviceStrategy string

const (
	SameDeviceReplace SameDeviceStrategy = "replace"
	SameDeviceReject  SameDeviceStrategy = "reject"
	SameDeviceAllow   SameDeviceStrategy = "allow"
)

type OverflowStrategy string

const (
	OverflowRevokeOldest OverflowStrategy = "revoke_oldest"
	OverflowReject       OverflowStrategy = "reject"
)

// Config controls access tokens, refresh tokens, session indexes, and local cache behavior.
type Config struct {
	Name                   string             `json:"name" toml:"name"`
	KeyPrefix              string             `json:"keyPrefix" toml:"keyPrefix"`
	JWTSignKey             string             `json:"jwtSignKey" toml:"jwtSignKey"`
	AccessTokenTTL         time.Duration      `json:"accessTokenTTL" toml:"accessTokenTTL"`
	AccessTokenDisplaySkew time.Duration      `json:"accessTokenDisplaySkew" toml:"accessTokenDisplaySkew"`
	RefreshTokenTTL        time.Duration      `json:"refreshTokenTTL" toml:"refreshTokenTTL"`
	RevokedRecordTTL       time.Duration      `json:"revokedRecordTTL" toml:"revokedRecordTTL"`
	TouchInterval          time.Duration      `json:"touchInterval" toml:"touchInterval"`
	MultiLoginEnabled      bool               `json:"multiLoginEnabled" toml:"multiLoginEnabled"`
	MaxSessions            int                `json:"maxSessions" toml:"maxSessions"`
	SameDeviceStrategy     SameDeviceStrategy `json:"sameDeviceStrategy" toml:"sameDeviceStrategy"`
	OverflowStrategy       OverflowStrategy   `json:"overflowStrategy" toml:"overflowStrategy"`
}

func DefaultConfig() *Config {
	return &Config{
		Name:                   "default",
		KeyPrefix:              "",
		JWTSignKey:             "",
		AccessTokenTTL:         2 * time.Hour,
		AccessTokenDisplaySkew: 30 * time.Minute,
		RefreshTokenTTL:        30 * 24 * time.Hour,
		RevokedRecordTTL:       24 * time.Hour,
		TouchInterval:          time.Minute,
		MultiLoginEnabled:      true,
		MaxSessions:            0,
		SameDeviceStrategy:     SameDeviceReplace,
		OverflowStrategy:       OverflowRevokeOldest,
	}
}

func (c *Config) normalize() {
	defaults := DefaultConfig()
	if c.Name == "" {
		c.Name = defaults.Name
	}
	if c.AccessTokenTTL <= 0 {
		c.AccessTokenTTL = defaults.AccessTokenTTL
	}
	if c.AccessTokenDisplaySkew < 0 {
		c.AccessTokenDisplaySkew = 0
	}
	if c.AccessTokenDisplaySkew >= c.AccessTokenTTL {
		c.AccessTokenDisplaySkew = 0
	}
	if c.RefreshTokenTTL <= 0 {
		c.RefreshTokenTTL = defaults.RefreshTokenTTL
	}
	if c.RevokedRecordTTL <= 0 {
		c.RevokedRecordTTL = defaults.RevokedRecordTTL
	}
	if c.TouchInterval <= 0 {
		c.TouchInterval = defaults.TouchInterval
	}
	switch c.SameDeviceStrategy {
	case SameDeviceReplace, SameDeviceReject, SameDeviceAllow:
	default:
		c.SameDeviceStrategy = defaults.SameDeviceStrategy
	}
	switch c.OverflowStrategy {
	case OverflowRevokeOldest, OverflowReject:
	default:
		c.OverflowStrategy = defaults.OverflowStrategy
	}
}
