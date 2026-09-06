// Command schema applies sql/001_schema.sql to one or both twins.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kleinron/uuid-insert-experiment/internal/app"
	"github.com/kleinron/uuid-insert-experiment/internal/db"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if err := run(); err != nil {
		log.Fatalf("schema: %v", err)
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

	for _, h := range hosts {
		log.Printf("apply %s on arm=%s host=%s", cfg.SchemaFile, h.Arm, h.Host)
		if err := db.ApplySchemaFile(ctx, h.DB, cfg.SchemaFile); err != nil {
			return fmt.Errorf("host %s: %w", h.Host, err)
		}
	}
	return nil
}

func init() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Fprintf(os.Stderr, "usage: schema [-both]\n")
		os.Exit(0)
	}
}
