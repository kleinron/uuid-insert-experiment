// Command loadgen runs the warmup or measure insert protocol against one twin.
//
//	EXPERIMENT_ARM=v4|v7 selects MYSQL_HOST_V4 or MYSQL_HOST_V7 and the UUID type.
//	-mode=warmup  full-rate inserts, discard histograms, then SETTLE_SECONDS pause
//	-mode=measure full-rate inserts, HDR p50/p95/p99/p999 written to results/
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kleinron/uuid-insert-experiment/internal/app"
	"github.com/kleinron/uuid-insert-experiment/internal/db"
	"github.com/kleinron/uuid-insert-experiment/internal/metrics"
	"github.com/kleinron/uuid-insert-experiment/internal/runner"
	"github.com/kleinron/uuid-insert-experiment/internal/uuidx"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if err := run(); err != nil {
		log.Fatalf("loadgen: %v", err)
	}
}

func run() error {
	mode := app.ArgFlag("mode", "measure")
	if mode != "warmup" && mode != "measure" {
		return fmt.Errorf("mode must be warmup or measure, got %q", mode)
	}

	ctx, cancel := app.SignalContext()
	defer cancel()

	cfg, err := app.LoadResolved(ctx)
	if err != nil {
		return err
	}
	host, err := cfg.Host()
	if err != nil {
		return err
	}

	sqldb, err := db.Open(cfg, host, 0)
	if err != nil {
		return err
	}
	defer sqldb.Close()

	pk := func() ([16]byte, error) { return uuidx.ForArm(cfg.Arm) }

	switch mode {
	case "warmup":
		log.Printf("warmup arm=%s host=%s duration=%s settle=%s qps=%.1f pool=%d",
			cfg.Arm, host, cfg.WarmupDuration(), cfg.SettleDuration(), cfg.TargetQPS, cfg.PoolSize)
		if _, err := runner.Run(ctx, sqldb, cfg, pk, cfg.WarmupDuration(), cfg.TargetQPS, false); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d := cfg.SettleDuration(); d > 0 {
			log.Printf("client settle %s (end of warmup)", d)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(d):
			}
		}
		log.Printf("warmup complete; histograms discarded")
		return nil
	default:
		log.Printf("measure arm=%s host=%s duration=%s qps=%.1f pool=%d",
			cfg.Arm, host, cfg.MeasureDuration(), cfg.TargetQPS, cfg.PoolSize)
		start := time.Now()
		st, err := runner.Run(ctx, sqldb, cfg, pk, cfg.MeasureDuration(), cfg.TargetQPS, true)
		if err != nil {
			return err
		}
		end := time.Now()
		elapsed := end.Sub(start).Seconds()
		actual := 0.0
		if elapsed > 0 {
			actual = float64(st.Inserts) / elapsed
		}
		res := metrics.Result{
			Arm:             cfg.Arm,
			Host:            host,
			StartedAt:       start.UTC(),
			EndedAt:         end.UTC(),
			DurationSeconds: elapsed,
			TargetQPS:       cfg.TargetQPS,
			ActualQPS:       actual,
			Inserts:         st.Inserts,
			Errors:          st.Errors,
			PoolSize:        cfg.PoolSize,
			LatencyUS:       st.Hist.Snapshot(),
		}
		path, err := metrics.Write(cfg.ResultsDir, res)
		if err != nil {
			return err
		}
		log.Printf("wrote %s  p50=%dµs p95=%dµs p99=%dµs p999=%dµs",
			path, res.LatencyUS.P50, res.LatencyUS.P95, res.LatencyUS.P99, res.LatencyUS.P999)
		return nil
	}
}

func init() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Fprintf(os.Stderr, "usage: loadgen -mode=warmup|measure\n")
		os.Exit(0)
	}
}
