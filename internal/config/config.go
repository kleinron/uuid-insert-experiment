// Package config loads harness settings from the environment and an optional .env file.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	DefaultPort              = 3306
	DefaultDatabase          = "payments_exp"
	DefaultUser              = "exp_app"
	DefaultTargetQPS         = 500
	DefaultPoolSize          = 32
	DefaultWarmupMinutes     = 25
	DefaultMeasureMinutes    = 20
	DefaultSettleSeconds     = 120
	DefaultPreloadBulkRows   = 185_000_000
	DefaultPreloadSeasonRows = 15_000_000
	DefaultPreloadBatchSize  = 1000
	DefaultPreloadWorkers    = 8
	DefaultPreloadMode       = "batch"
	DefaultResultsDir        = "results"
	DefaultSchemaFile        = "sql/001_schema.sql"
)

// Config is the full harness configuration. Password may be empty until ResolvePassword.
type Config struct {
	HostV4   string
	HostV7   string
	Port     int
	Database string
	User     string

	// Password is set from MYSQL_PASSWORD or resolved from Secrets Manager.
	Password  string
	SecretARN string
	AWSRegion string

	Arm string // v4 | v7

	TargetQPS      float64
	PoolSize       int
	WarmupMinutes  float64
	MeasureMinutes float64
	SettleSeconds  float64

	PreloadBulkRows   uint64
	PreloadSeasonRows uint64
	PreloadBatchSize  int
	PreloadWorkers    int
	PreloadMode       string // batch | loaddata
	PreloadStart      uint64

	SeasonQPS float64 // 0 = unlimited batched inserts

	ResultsDir string
	SchemaFile string
}

// Load reads .env (if present) then environment variables.
// Existing process env wins over .env. Missing .env is not an error.
func Load() (*Config, error) {
	_ = godotenv.Load()

	c := &Config{
		HostV4:            strings.TrimSpace(os.Getenv("MYSQL_HOST_V4")),
		HostV7:            strings.TrimSpace(os.Getenv("MYSQL_HOST_V7")),
		Port:              envInt("MYSQL_PORT", DefaultPort),
		Database:          envStr("MYSQL_DATABASE", DefaultDatabase),
		User:              envStr("MYSQL_USER", DefaultUser),
		Password:          os.Getenv("MYSQL_PASSWORD"),
		SecretARN:         strings.TrimSpace(os.Getenv("MYSQL_SECRET_ARN")),
		AWSRegion:         envStr("AWS_REGION", "us-east-1"),
		Arm:               strings.ToLower(strings.TrimSpace(envStr("EXPERIMENT_ARM", "v4"))),
		TargetQPS:         envFloat("TARGET_QPS", DefaultTargetQPS),
		PoolSize:          envInt("POOL_SIZE", DefaultPoolSize),
		WarmupMinutes:     envFloat("WARMUP_MINUTES", DefaultWarmupMinutes),
		MeasureMinutes:    envFloat("MEASURE_MINUTES", DefaultMeasureMinutes),
		SettleSeconds:     envFloat("SETTLE_SECONDS", DefaultSettleSeconds),
		PreloadBulkRows:   envUint64("PRELOAD_BULK_ROWS", DefaultPreloadBulkRows),
		PreloadSeasonRows: envUint64("PRELOAD_SEASON_ROWS", DefaultPreloadSeasonRows),
		PreloadBatchSize:  envInt("PRELOAD_BATCH_SIZE", DefaultPreloadBatchSize),
		PreloadWorkers:    envInt("PRELOAD_WORKERS", DefaultPreloadWorkers),
		PreloadMode:       strings.ToLower(envStr("PRELOAD_MODE", DefaultPreloadMode)),
		PreloadStart:      envUint64("PRELOAD_START_OFFSET", 0),
		SeasonQPS:         envFloat("SEASON_QPS", 0),
		ResultsDir:        envStr("RESULTS_DIR", DefaultResultsDir),
		SchemaFile:        envStr("SCHEMA_FILE", DefaultSchemaFile),
	}
	if err := c.validateStatic(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validateStatic() error {
	if c.Arm != "v4" && c.Arm != "v7" {
		return fmt.Errorf("EXPERIMENT_ARM must be v4 or v7, got %q", c.Arm)
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("MYSQL_PORT out of range: %d", c.Port)
	}
	if c.Database == "" || c.User == "" {
		return fmt.Errorf("MYSQL_DATABASE and MYSQL_USER are required")
	}
	if c.TargetQPS <= 0 {
		return fmt.Errorf("TARGET_QPS must be > 0")
	}
	if c.PoolSize < 1 {
		return fmt.Errorf("POOL_SIZE must be >= 1")
	}
	if c.WarmupMinutes < 0 || c.MeasureMinutes < 0 || c.SettleSeconds < 0 {
		return fmt.Errorf("WARMUP_MINUTES, MEASURE_MINUTES, SETTLE_SECONDS must be >= 0")
	}
	if c.PreloadBatchSize < 1 {
		return fmt.Errorf("PRELOAD_BATCH_SIZE must be >= 1")
	}
	if c.PreloadWorkers < 1 {
		return fmt.Errorf("PRELOAD_WORKERS must be >= 1")
	}
	if c.PreloadMode != "batch" && c.PreloadMode != "loaddata" {
		return fmt.Errorf("PRELOAD_MODE must be batch or loaddata, got %q", c.PreloadMode)
	}
	return nil
}

// Host returns the MySQL hostname for the current experiment arm.
func (c *Config) Host() (string, error) {
	return c.HostForArm(c.Arm)
}

// HostForArm returns MYSQL_HOST_V4 or MYSQL_HOST_V7.
func (c *Config) HostForArm(arm string) (string, error) {
	switch arm {
	case "v4":
		if c.HostV4 == "" {
			return "", fmt.Errorf("MYSQL_HOST_V4 is required when targeting the v4 twin")
		}
		return c.HostV4, nil
	case "v7":
		if c.HostV7 == "" {
			return "", fmt.Errorf("MYSQL_HOST_V7 is required when targeting the v7 twin")
		}
		return c.HostV7, nil
	default:
		return "", fmt.Errorf("unknown arm %q", arm)
	}
}

// Hosts returns one or both twin hostnames.
func (c *Config) Hosts(both bool) ([]namedHost, error) {
	if !both {
		h, err := c.Host()
		if err != nil {
			return nil, err
		}
		return []namedHost{{Arm: c.Arm, Host: h}}, nil
	}
	var out []namedHost
	if c.HostV4 != "" {
		out = append(out, namedHost{Arm: "v4", Host: c.HostV4})
	}
	if c.HostV7 != "" {
		out = append(out, namedHost{Arm: "v7", Host: c.HostV7})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("MYSQL_HOST_V4 and/or MYSQL_HOST_V7 required")
	}
	return out, nil
}

// NamedHost is a twin endpoint.
type namedHost struct {
	Arm  string
	Host string
}

// NamedHost is exported for command packages.
type NamedHost = namedHost

// WarmupDuration is WARMUP_MINUTES as a duration (fractional minutes allowed).
func (c *Config) WarmupDuration() time.Duration {
	return minutes(c.WarmupMinutes)
}

// MeasureDuration is MEASURE_MINUTES as a duration (fractional minutes allowed).
func (c *Config) MeasureDuration() time.Duration {
	return minutes(c.MeasureMinutes)
}

// SettleDuration is SETTLE_SECONDS as a duration.
func (c *Config) SettleDuration() time.Duration {
	return time.Duration(c.SettleSeconds * float64(time.Second))
}

func minutes(m float64) time.Duration {
	return time.Duration(m * float64(time.Minute))
}

// PasswordSource describes how the DB password will be obtained.
func (c *Config) PasswordSource() string {
	if strings.TrimSpace(c.Password) != "" {
		return "MYSQL_PASSWORD (local override)"
	}
	if c.SecretARN != "" {
		return "MYSQL_SECRET_ARN (AWS Secrets Manager)"
	}
	return "missing (set MYSQL_PASSWORD or MYSQL_SECRET_ARN)"
}

// HasPassword returns whether a local override is already present.
func (c *Config) HasPassword() bool {
	return strings.TrimSpace(c.Password) != ""
}

func envStr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envFloat(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func envUint64(key string, def uint64) uint64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
