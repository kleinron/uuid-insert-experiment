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
	for _, k := range []string{"MYSQL_PORT", "TARGET_QPS", "POOL_SIZE", "MYSQL_TLS", "READER_POOL_SIZE", "READER_QPS", "READERS", "PRELOAD_SEASON_ROWS"} {
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
	if c.ReaderPoolSize != DefaultReaderPoolSize || c.ReaderQPS != DefaultReaderQPS {
		t.Fatalf("reader defaults: pool=%d qps=%v", c.ReaderPoolSize, c.ReaderQPS)
	}
	if !c.ReadersEnabled() || c.PreloadKeySpace() != 1000+DefaultPreloadSeasonRows {
		t.Fatalf("readers enabled=%v key_space=%d", c.ReadersEnabled(), c.PreloadKeySpace())
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
	if c.TLS {
		t.Fatal("MYSQL_TLS default should be false")
	}
}

func TestLoadMYSQLTLS(t *testing.T) {
	t.Setenv("MYSQL_TLS", "true")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.TLS {
		t.Fatal("MYSQL_TLS=true not parsed")
	}
	t.Setenv("MYSQL_TLS", "false")
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.TLS {
		t.Fatal("MYSQL_TLS=false should disable TLS")
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

func TestReaderPoolSizeZeroDisables(t *testing.T) {
	t.Setenv("MYSQL_HOST_V4", "v4.example.internal")
	t.Setenv("MYSQL_PASSWORD", "dev-only")
	t.Setenv("READER_POOL_SIZE", "0")
	t.Setenv("READER_QPS", "1000")
	_ = os.Unsetenv("READERS")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ReadersEnabled() || c.ReaderPoolSize != 0 {
		t.Fatalf("expected readers disabled, pool=%d", c.ReaderPoolSize)
	}
	if c.ReaderQPS != 1000 {
		t.Fatalf("reader qps=%v", c.ReaderQPS)
	}
}

func TestReadersEnvZeroDisables(t *testing.T) {
	t.Setenv("MYSQL_HOST_V4", "v4.example.internal")
	t.Setenv("MYSQL_PASSWORD", "dev-only")
	t.Setenv("READER_POOL_SIZE", "16")
	t.Setenv("READERS", "0")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ReadersEnabled() || c.ReaderPoolSize != 0 {
		t.Fatalf("READERS=0 should disable, pool=%d", c.ReaderPoolSize)
	}
}

func TestReadersEnvFalseDisables(t *testing.T) {
	t.Setenv("MYSQL_HOST_V4", "v4.example.internal")
	t.Setenv("MYSQL_PASSWORD", "dev-only")
	t.Setenv("READER_POOL_SIZE", "8")
	t.Setenv("READERS", "false")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ReadersEnabled() {
		t.Fatal("READERS=false should disable")
	}
}

func TestReaderQPSRequiredWhenEnabled(t *testing.T) {
	t.Setenv("MYSQL_HOST_V4", "v4.example.internal")
	t.Setenv("MYSQL_PASSWORD", "dev-only")
	t.Setenv("READER_POOL_SIZE", "8")
	t.Setenv("READER_QPS", "0")
	_ = os.Unsetenv("READERS")

	if _, err := Load(); err == nil {
		t.Fatal("expected READER_QPS validation error")
	}
}

func TestReadersRequirePreloadKeySpace(t *testing.T) {
	t.Setenv("MYSQL_HOST_V4", "v4.example.internal")
	t.Setenv("MYSQL_PASSWORD", "dev-only")
	t.Setenv("READER_POOL_SIZE", "4")
	t.Setenv("PRELOAD_BULK_ROWS", "0")
	t.Setenv("PRELOAD_SEASON_ROWS", "0")
	_ = os.Unsetenv("READERS")

	if _, err := Load(); err == nil {
		t.Fatal("expected key space validation error")
	}
}

func TestPreloadKeySpace(t *testing.T) {
	c := &Config{PreloadBulkRows: 10, PreloadSeasonRows: 5}
	if c.PreloadKeySpace() != 15 {
		t.Fatalf("key space %d", c.PreloadKeySpace())
	}
}
