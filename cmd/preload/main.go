// Command preload bulk-loads deterministic rows (shared non-arm key source).
//
// Uses PRELOAD_BULK_ROWS starting at PRELOAD_START_OFFSET (default 0).
// Pass -both to load identical rows onto both twins.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kleinron/uuid-insert-experiment/internal/app"
	"github.com/kleinron/uuid-insert-experiment/internal/bulk"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if err := run(); err != nil {
		log.Fatalf("preload: %v", err)
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

	start := cfg.PreloadStart
	count := cfg.PreloadBulkRows
	log.Printf("preload-bulk rows=%d start=%d mode=%s both=%v", count, start, cfg.PreloadMode, both)
	for _, h := range hosts {
		log.Printf("preload-bulk target arm=%s host=%s (keys are not v4/v7)", h.Arm, h.Host)
		if err := bulk.InsertRange(ctx, h.DB, cfg, start, count); err != nil {
			return fmt.Errorf("host %s: %w", h.Host, err)
		}
	}
	return nil
}

func init() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Fprintf(os.Stderr, "usage: preload [-both]\n")
		os.Exit(0)
	}
}
