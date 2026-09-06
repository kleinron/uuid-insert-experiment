// Command exportmetrics prints a table of results/*.json measure files.
package main

import (
	"fmt"
	"os"

	"github.com/kleinron/uuid-insert-experiment/internal/config"
	"github.com/kleinron/uuid-insert-experiment/internal/metrics"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "export-metrics: %v\n", err)
		os.Exit(1)
	}
	results, err := metrics.LoadDir(cfg.ResultsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "export-metrics: %v\n", err)
		os.Exit(1)
	}
	metrics.FprintTable(os.Stdout, results)
}
