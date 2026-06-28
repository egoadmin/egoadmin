package idgen

import (
	"fmt"
	"time"
)

const (
	// PackageName identifies this component in EGO logs and metrics.
	PackageName = "component.idgen"

	defaultComponentName = "component.idgen.default"
)

// LostPolicy controls behavior when the optional machine lease is lost.
type LostPolicy string

const (
	LostPolicyDegraded   LostPolicy = "degraded"
	LostPolicyFailClosed LostPolicy = "fail_closed"
)

// Config contains runtime settings for the ID generator component.
type Config struct {
	Namespace              string        `json:"namespace" toml:"namespace"`
	Name                   string        `json:"name" toml:"name"`
	Step                   int64         `json:"step" toml:"step"`
	MinStep                int64         `json:"minStep" toml:"minStep"`
	MaxStep                int64         `json:"maxStep" toml:"maxStep"`
	AutoEnsure             bool          `json:"autoEnsure" toml:"autoEnsure"`
	Warmup                 bool          `json:"warmup" toml:"warmup"`
	FetchTimeout           time.Duration `json:"fetchTimeout" toml:"fetchTimeout"`
	WaitTimeout            time.Duration `json:"waitTimeout" toml:"waitTimeout"`
	PrefetchRemainingRatio float64       `json:"prefetchRemainingRatio" toml:"prefetchRemainingRatio"`
	DynamicStep            bool          `json:"dynamicStep" toml:"dynamicStep"`
	TargetDuration         time.Duration `json:"targetDuration" toml:"targetDuration"`
	MaxPrefetchWorkers     int           `json:"maxPrefetchWorkers" toml:"maxPrefetchWorkers"`
	EnableMetrics          bool          `json:"enableMetrics" toml:"enableMetrics"`
	EnableNameMetricLabel  bool          `json:"enableNameMetricLabel" toml:"enableNameMetricLabel"`
}

// MachineConfig controls the process-level machine lease.
type MachineConfig struct {
	Group             string        `json:"group" toml:"group"`
	MaxMachineID      int           `json:"maxMachineID" toml:"maxMachineID"`
	TTL               time.Duration `json:"ttl" toml:"ttl"`
	RenewInterval     time.Duration `json:"renewInterval" toml:"renewInterval"`
	RenewTimeout      time.Duration `json:"renewTimeout" toml:"renewTimeout"`
	MinRenewWindows   int           `json:"minRenewWindows" toml:"minRenewWindows"`
	ReallocateBackoff time.Duration `json:"reallocateBackoff" toml:"reallocateBackoff"`
	StableInstanceID  string        `json:"stableInstanceID" toml:"stableInstanceID"`
	LostPolicy        LostPolicy    `json:"lostPolicy" toml:"lostPolicy"`
}

// DefaultConfig returns conservative defaults suitable for development.
func DefaultConfig() *Config {
	return &Config{
		Namespace:              "default",
		Name:                   "default",
		Step:                   100000,
		MinStep:                10000,
		MaxStep:                100000000,
		AutoEnsure:             true,
		Warmup:                 true,
		FetchTimeout:           2 * time.Second,
		WaitTimeout:            200 * time.Millisecond,
		PrefetchRemainingRatio: 0.2,
		DynamicStep:            true,
		TargetDuration:         15 * time.Minute,
		MaxPrefetchWorkers:     8,
		EnableMetrics:          true,
		EnableNameMetricLabel:  false,
	}
}

func DefaultMachineConfig() *MachineConfig {
	return &MachineConfig{
		Group:             "default",
		MaxMachineID:      1023,
		TTL:               60 * time.Second,
		RenewInterval:     10 * time.Second,
		RenewTimeout:      5 * time.Second,
		MinRenewWindows:   5,
		ReallocateBackoff: 2 * time.Second,
		LostPolicy:        LostPolicyFailClosed,
	}
}

func (c *Config) normalize() {
	defaults := DefaultConfig()
	if c.Namespace == "" {
		c.Namespace = defaults.Namespace
	}
	if c.Name == "" {
		c.Name = defaults.Name
	}
	if c.Step <= 0 {
		c.Step = defaults.Step
	}
	if c.MinStep <= 0 {
		c.MinStep = defaults.MinStep
	}
	if c.MaxStep <= 0 {
		c.MaxStep = defaults.MaxStep
	}
	if c.MinStep > c.Step {
		c.Step = c.MinStep
	}
	if c.MaxStep < c.Step {
		c.MaxStep = c.Step
	}
	if c.FetchTimeout <= 0 {
		c.FetchTimeout = defaults.FetchTimeout
	}
	if c.WaitTimeout <= 0 {
		c.WaitTimeout = defaults.WaitTimeout
	}
	if c.PrefetchRemainingRatio <= 0 || c.PrefetchRemainingRatio >= 1 {
		c.PrefetchRemainingRatio = defaults.PrefetchRemainingRatio
	}
	if c.TargetDuration <= 0 {
		c.TargetDuration = defaults.TargetDuration
	}
	if c.MaxPrefetchWorkers <= 0 {
		c.MaxPrefetchWorkers = defaults.MaxPrefetchWorkers
	}
}

func (c *MachineConfig) normalize() {
	defaults := DefaultMachineConfig()
	if c.Group == "" {
		c.Group = defaults.Group
	}
	if c.MaxMachineID <= 0 {
		c.MaxMachineID = defaults.MaxMachineID
	}
	if c.TTL <= 0 {
		c.TTL = defaults.TTL
	}
	if c.RenewInterval <= 0 || c.RenewInterval >= c.TTL {
		c.RenewInterval = c.TTL / 3
	}
	if c.RenewInterval <= 0 {
		c.RenewInterval = defaults.RenewInterval
	}
	if c.MinRenewWindows <= 0 {
		c.MinRenewWindows = defaults.MinRenewWindows
	}
	minTTL := time.Duration(c.MinRenewWindows+1) * c.RenewInterval
	if c.TTL < minTTL {
		c.TTL = minTTL
	}
	if c.RenewTimeout <= 0 || c.RenewTimeout >= c.RenewInterval {
		c.RenewTimeout = defaults.RenewTimeout
	}
	if c.RenewTimeout >= c.RenewInterval {
		c.RenewTimeout = c.RenewInterval / 2
	}
	if c.RenewTimeout <= 0 {
		c.RenewTimeout = defaults.RenewTimeout
	}
	if c.ReallocateBackoff <= 0 {
		c.ReallocateBackoff = defaults.ReallocateBackoff
	}
	if c.LostPolicy == "" {
		c.LostPolicy = defaults.LostPolicy
	}
}

func (c *Config) validate() error {
	if c == nil {
		return fmt.Errorf("%w: config is nil", ErrInvalidConfig)
	}
	if c.Namespace == "" {
		return fmt.Errorf("%w: namespace is empty", ErrInvalidConfig)
	}
	if c.Name == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidConfig)
	}
	if c.Step <= 0 {
		return fmt.Errorf("%w: step must be positive", ErrInvalidConfig)
	}
	if c.MinStep <= 0 {
		return fmt.Errorf("%w: minStep must be positive", ErrInvalidConfig)
	}
	if c.MaxStep <= 0 {
		return fmt.Errorf("%w: maxStep must be positive", ErrInvalidConfig)
	}
	if c.MinStep > c.MaxStep {
		return fmt.Errorf("%w: minStep must be less than or equal to maxStep", ErrInvalidConfig)
	}
	if c.Step < c.MinStep || c.Step > c.MaxStep {
		return fmt.Errorf("%w: step must be between minStep and maxStep", ErrInvalidConfig)
	}
	if c.FetchTimeout <= 0 {
		return fmt.Errorf("%w: fetchTimeout must be positive", ErrInvalidConfig)
	}
	if c.WaitTimeout <= 0 {
		return fmt.Errorf("%w: waitTimeout must be positive", ErrInvalidConfig)
	}
	if c.PrefetchRemainingRatio <= 0 || c.PrefetchRemainingRatio >= 1 {
		return fmt.Errorf("%w: prefetchRemainingRatio must be between 0 and 1", ErrInvalidConfig)
	}
	if c.TargetDuration <= 0 {
		return fmt.Errorf("%w: targetDuration must be positive", ErrInvalidConfig)
	}
	if c.MaxPrefetchWorkers <= 0 {
		return fmt.Errorf("%w: maxPrefetchWorkers must be positive", ErrInvalidConfig)
	}
	return nil
}

func (c *MachineConfig) validate() error {
	if c == nil {
		return fmt.Errorf("%w: machine config is nil", ErrInvalidConfig)
	}
	if c.Group == "" {
		return fmt.Errorf("%w: machine group is empty", ErrInvalidConfig)
	}
	if c.MaxMachineID < 0 {
		return fmt.Errorf("%w: maxMachineID must be non-negative", ErrInvalidConfig)
	}
	if c.TTL <= 0 {
		return fmt.Errorf("%w: machine ttl must be positive", ErrInvalidConfig)
	}
	if c.RenewInterval <= 0 || c.RenewInterval >= c.TTL {
		return fmt.Errorf("%w: renewInterval must be positive and less than ttl", ErrInvalidConfig)
	}
	if c.MinRenewWindows < 1 {
		return fmt.Errorf("%w: minRenewWindows must be positive", ErrInvalidConfig)
	}
	if c.TTL < time.Duration(c.MinRenewWindows+1)*c.RenewInterval {
		return fmt.Errorf("%w: machine ttl must leave configured renew windows", ErrInvalidConfig)
	}
	if c.RenewTimeout <= 0 || c.RenewTimeout >= c.RenewInterval {
		return fmt.Errorf("%w: renewTimeout must be positive and less than renewInterval", ErrInvalidConfig)
	}
	if c.ReallocateBackoff <= 0 {
		return fmt.Errorf("%w: reallocateBackoff must be positive", ErrInvalidConfig)
	}
	switch c.LostPolicy {
	case LostPolicyDegraded, LostPolicyFailClosed:
	default:
		return fmt.Errorf("%w: unsupported lostPolicy %q", ErrInvalidConfig, c.LostPolicy)
	}
	return nil
}
