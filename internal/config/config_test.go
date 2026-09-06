package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaultsAndArmHost(t *testing.T) {
	t.Setenv("MYSQL_HOST_V4", "v4.example.internal")
	t.Setenv("MYSQL_HOST_V7", "v7.example.internal")
	t.Setenv("MYSQL_PASSWORD", "dev-only")
	t.Setenv("EXPERIMENT_ARM", "v7")
	t.Setenv("WARMUP_MINUTES", "0.05")
	t.Setenv("PRELOAD_BULK_ROWS", "1000")
	// Clear others that might leak from the environment.
	for _, k := range []string{"MYSQL_PORT", "TARGET_QPS", "POOL_SIZE"} {
		_ = os.Unsetenv(k)
	}

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Arm != "v7" {
		t.Fatalf("arm=%s", c.Arm)
	}
	h, err := c.Host()
	if err != nil {
		t.Fatal(err)
	}
	if h != "v7.example.internal" {
		t.Fatalf("host=%s", h)
	}
	if c.Port != DefaultPort || c.TargetQPS != DefaultTargetQPS || c.PoolSize != DefaultPoolSize {
		t.Fatalf("defaults: port=%d qps=%v pool=%d", c.Port, c.TargetQPS, c.PoolSize)
	}
	if c.WarmupDuration() != 3*time.Second {
		t.Fatalf("warmup duration %s", c.WarmupDuration())
	}
	if c.PreloadBulkRows != 1000 {
		t.Fatalf("bulk rows %d", c.PreloadBulkRows)
	}
	if c.PasswordSource() != "MYSQL_PASSWORD (local override)" {
		t.Fatalf("source %s", c.PasswordSource())
	}
}

func TestLoadRejectsBadArm(t *testing.T) {
	t.Setenv("EXPERIMENT_ARM", "v1")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestHostsBoth(t *testing.T) {
	c := &Config{HostV4: "a", HostV7: "b", Arm: "v4"}
	hs, err := c.Hosts(true)
	if err != nil || len(hs) != 2 {
		t.Fatalf("hosts=%v err=%v", hs, err)
	}
}

func TestHostMissing(t *testing.T) {
	c := &Config{Arm: "v4"}
	if _, err := c.Host(); err == nil {
		t.Fatal("expected missing host error")
	}
}
