// Command configcheck validates environment configuration without inventing secrets.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kleinron/uuid-insert-experiment/internal/config"
	"github.com/kleinron/uuid-insert-experiment/internal/db"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "config-check: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Println("uuid-insert-experiment config")
	fmt.Println("-----------------------------")
	fmt.Printf("EXPERIMENT_ARM       %s\n", cfg.Arm)
	fmt.Printf("MYSQL_HOST_V4        %s\n", empty(cfg.HostV4))
	fmt.Printf("MYSQL_HOST_V7        %s\n", empty(cfg.HostV7))
	fmt.Printf("MYSQL_PORT           %d\n", cfg.Port)
	fmt.Printf("MYSQL_DATABASE       %s\n", cfg.Database)
	fmt.Printf("MYSQL_USER           %s\n", cfg.User)
	fmt.Printf("password source      %s\n", cfg.PasswordSource())
	fmt.Printf("MYSQL_SECRET_ARN     %s\n", empty(cfg.SecretARN))
	fmt.Printf("AWS_REGION           %s\n", cfg.AWSRegion)
	fmt.Printf("MYSQL_TLS            %v\n", cfg.TLS)
	fmt.Printf("TARGET_QPS           %.1f\n", cfg.TargetQPS)
	fmt.Printf("POOL_SIZE            %d\n", cfg.PoolSize)
	fmt.Printf("WARMUP_MINUTES       %.4f (%s)\n", cfg.WarmupMinutes, cfg.WarmupDuration())
	fmt.Printf("MEASURE_MINUTES      %.4f (%s)\n", cfg.MeasureMinutes, cfg.MeasureDuration())
	fmt.Printf("SETTLE_SECONDS       %.1f\n", cfg.SettleSeconds)
	fmt.Printf("PRELOAD_BULK_ROWS    %d\n", cfg.PreloadBulkRows)
	fmt.Printf("PRELOAD_SEASON_ROWS  %d\n", cfg.PreloadSeasonRows)
	fmt.Printf("PRELOAD_MODE         %s\n", cfg.PreloadMode)
	fmt.Printf("PRELOAD_BATCH_SIZE   %d\n", cfg.PreloadBatchSize)
	fmt.Printf("PRELOAD_WORKERS      %d\n", cfg.PreloadWorkers)
	fmt.Println()

	if cfg.PreloadBulkRows < 1_000_000 {
		fmt.Println("note: PRELOAD_BULK_ROWS < 1e6 — small-scale / test mode")
	} else if cfg.PreloadBulkRows >= 100_000_000 {
		fmt.Println("note: PRELOAD_BULK_ROWS is full-experiment sized")
	}

	if !cfg.HasPassword() && cfg.SecretARN == "" {
		return fmt.Errorf("no password configured. For local/dev set MYSQL_PASSWORD. For RDS set MYSQL_SECRET_ARN (JSON secret with a password field) and AWS credentials")
	}

	if !cfg.HasPassword() {
		fmt.Println("resolving password from AWS Secrets Manager (no local MYSQL_PASSWORD)...")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := cfg.ResolvePassword(ctx); err != nil {
			return fmt.Errorf("%w\n  (for a no-AWS smoke run, set MYSQL_PASSWORD instead)", err)
		}
		fmt.Println("Secrets Manager: password fetched")
	}

	if os.Getenv("CONFIG_CHECK_PING") == "0" {
		fmt.Println("CONFIG_CHECK_PING=0 — skipping database ping")
		return nil
	}

	// Ping the current arm if its host is set; also ping the other twin when present.
	var pinged int
	for _, arm := range []string{"v4", "v7"} {
		host, err := cfg.HostForArm(arm)
		if err != nil {
			if arm == cfg.Arm {
				return err
			}
			fmt.Printf("skip ping %s: %v\n", arm, err)
			continue
		}
		sqldb, err := db.Open(cfg, host, 1)
		if err != nil {
			return fmt.Errorf("ping %s (%s): %w", arm, host, err)
		}
		_ = sqldb.Close()
		fmt.Printf("ping ok  arm=%s host=%s\n", arm, host)
		pinged++
	}
	if pinged == 0 {
		return fmt.Errorf("no MYSQL_HOST_* set to ping")
	}
	fmt.Println("config-check passed")
	return nil
}

func empty(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}
