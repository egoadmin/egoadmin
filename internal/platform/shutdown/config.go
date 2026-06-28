package shutdown

import "time"

const (
	defaultStopTimeout  = 20 * time.Second
	defaultDrainTimeout = 2 * time.Second
	defaultCloseTimeout = 5 * time.Second
)

// Config controls process shutdown behavior.
type Config struct {
	StopTimeout  time.Duration `toml:"stopTimeout"`
	DrainTimeout time.Duration `toml:"drainTimeout"`
	CloseTimeout time.Duration `toml:"closeTimeout"`
}

func (c Config) Normalize() Config {
	if c.StopTimeout <= 0 {
		c.StopTimeout = defaultStopTimeout
	}
	if c.DrainTimeout < 0 {
		c.DrainTimeout = 0
	}
	if c.DrainTimeout == 0 {
		c.DrainTimeout = defaultDrainTimeout
	}
	if c.CloseTimeout <= 0 {
		c.CloseTimeout = defaultCloseTimeout
	}
	if c.DrainTimeout > c.StopTimeout {
		c.DrainTimeout = c.StopTimeout
	}
	if c.CloseTimeout > c.StopTimeout {
		c.CloseTimeout = c.StopTimeout
	}
	return c
}

func DefaultConfig() Config {
	return Config{}.Normalize()
}
