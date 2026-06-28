package logincrypto

import "time"

const (
	// PackageName identifies the component in EGO logs.
	PackageName = "component.logincrypto"

	AlgorithmRSAOAEP256 = "RSA-OAEP-SHA256"

	ActionLogin              = "login"
	ActionCenterEditPassword = "center.edit_password"
	ActionCenterEditInfo     = "center.edit_info"
)

type Config struct {
	KeyPrefix     string        `json:"keyPrefix" toml:"keyPrefix"`
	ChallengeTTL  time.Duration `json:"challengeTTL" toml:"challengeTTL"`
	TimestampSkew time.Duration `json:"timestampSkew" toml:"timestampSkew"`
	RSAKeyBits    int           `json:"rsaKeyBits" toml:"rsaKeyBits"`
	EnableMetrics bool          `json:"enableMetrics" toml:"enableMetrics"`
}

func DefaultConfig() *Config {
	return &Config{
		KeyPrefix:     "",
		ChallengeTTL:  3 * time.Minute,
		TimestampSkew: 2 * time.Minute,
		RSAKeyBits:    4096,
		EnableMetrics: true,
	}
}

func (c *Config) normalize() {
	defaults := DefaultConfig()
	if c.ChallengeTTL <= 0 {
		c.ChallengeTTL = defaults.ChallengeTTL
	}
	if c.TimestampSkew <= 0 {
		c.TimestampSkew = defaults.TimestampSkew
	}
	if c.RSAKeyBits < 2048 {
		c.RSAKeyBits = defaults.RSAKeyBits
	}
}
