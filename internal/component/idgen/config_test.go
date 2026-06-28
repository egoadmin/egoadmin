package idgen

import (
	"testing"
	"time"
)

func TestConfigDefaultsMatchTemplateBehavior(t *testing.T) {
	cfg := DefaultConfig()
	cfg.normalize()
	if cfg.Namespace != "default" || cfg.Name != "default" {
		t.Fatalf("namespace/name = %q/%q, want default/default", cfg.Namespace, cfg.Name)
	}
	if !cfg.AutoEnsure || !cfg.Warmup {
		t.Fatalf("autoEnsure/warmup = %v/%v, want true/true", cfg.AutoEnsure, cfg.Warmup)
	}
	if cfg.Step != 100000 || cfg.MinStep != 10000 || cfg.MaxStep != 100000000 {
		t.Fatalf("step config = %d/%d/%d", cfg.Step, cfg.MinStep, cfg.MaxStep)
	}
	if cfg.EnableNameMetricLabel {
		t.Fatal("enableNameMetricLabel should default false")
	}
}

func TestMachineConfigDefaultsMatchTemplateBehavior(t *testing.T) {
	cfg := DefaultMachineConfig()
	cfg.Group = "egoadmin"
	cfg.normalize()
	if cfg.Group != "egoadmin" {
		t.Fatalf("group = %q, want egoadmin", cfg.Group)
	}
	if cfg.MaxMachineID != 1023 || cfg.TTL != 60*time.Second || cfg.RenewInterval != 10*time.Second {
		t.Fatalf("machine config = %+v", cfg)
	}
	if cfg.RenewTimeout != 5*time.Second || cfg.MinRenewWindows != 5 || cfg.ReallocateBackoff != 2*time.Second {
		t.Fatalf("machine config = %+v", cfg)
	}
	if cfg.LostPolicy != LostPolicyFailClosed {
		t.Fatalf("lostPolicy = %q, want fail_closed", cfg.LostPolicy)
	}
}
