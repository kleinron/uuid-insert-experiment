// Command season appends the second preload phase with the same shared key source.
//
// Keys are KeyAt(PRELOAD_BULK_ROWS + i) so they never overlap the bulk range
// and stay identical across twins. Default is batched inserts (SEASON_QPS=0).
// Set SEASON_QPS > 0 for rate-limited single-row seasoning.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kleinron/uuid-insert-experiment/internal/app"
	"github.com/kleinron/uuid-insert-experiment/internal/bulk"
	"github.com/kleinron/uuid-insert-experiment/internal/runner"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if err := run(); err != nil {
		log.Fatalf("season: %v", err)
	}
}

func run() error {
	ctx, cancel := app.SignalContext()
	defer cancel()

	cfg, err := app.LoadResolved(ctx)
	if err != nil {
		return err
	}
	both := app.BoolFlag("both")
	hosts, err := app.OpenHosts(cfg, both, cfg.PreloadWorkers)
	if err != nil {
		return err
	}
	defer app.CloseAll(hosts)

	start := cfg.PreloadBulkRows
	count := cfg.PreloadSeasonRows
	log.Printf("preload-season rows=%d start=%d season_qps=%.1f both=%v", count, start, cfg.SeasonQPS, both)

	for _, h := range hosts {
		log.Printf("season target arm=%s host=%s (keys are not v4/v7)", h.Arm, h.Host)
		if cfg.SeasonQPS > 0 {
			pk := runner.DeterministicPK(start)
			dur := time.Duration(float64(count)/cfg.SeasonQPS+5) * time.Second
			if _, err := runner.Run(ctx, h.DB, cfg, pk, dur, cfg.SeasonQPS, false); err != nil {
				return fmt.Errorf("host %s: %w", h.Host, err)
			}
			continue
		}
		if err := bulk.InsertRange(ctx, h.DB, cfg, start, count); err != nil {
			return fmt.Errorf("host %s: %w", h.Host, err)
		}
	}
	return nil
}

func init() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Fprintf(os.Stderr, "usage: season [-both]\n")
		os.Exit(0)
	}
}
