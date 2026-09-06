// Command tablespace prints information_schema size stats for payments.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/kleinron/uuid-insert-experiment/internal/app"
	"github.com/kleinron/uuid-insert-experiment/internal/db"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if err := run(); err != nil {
		log.Fatalf("tablespace: %v", err)
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
	hosts, err := app.OpenHosts(cfg, both, 2)
	if err != nil {
		return err
	}
	defer app.CloseAll(hosts)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, h := range hosts {
		rep, err := db.Tablespace(ctx, h.DB, cfg.Database, "payments")
		if err != nil {
			return fmt.Errorf("host %s: %w", h.Host, err)
		}
		out := struct {
			Arm  string `json:"arm"`
			Host string `json:"host"`
			db.Report
		}{Arm: h.Arm, Host: h.Host, Report: rep}
		if err := enc.Encode(out); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "arm=%s host=%s rows≈%d data=%d index=%d free=%d avg_row=%d data+index=%d\n",
			h.Arm, h.Host, rep.TableRows, rep.DataLength, rep.IndexLength, rep.DataFree, rep.AvgRowLength, rep.DataAndIndexBytes())
		if rep.OverRevisitThreshold() {
			fmt.Fprintf(os.Stderr, "NOTE: measured tablespace ≥ 50 GiB (data+index=%d) — revisit instance class / InnoDB buffer pool (BP) before warmup/measure\n",
				rep.DataAndIndexBytes())
		}
	}
	return nil
}

func init() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Fprintf(os.Stderr, "usage: tablespace [-both]\n")
		os.Exit(0)
	}
}
