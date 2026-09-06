// Package app shares startup helpers across commands.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kleinron/uuid-insert-experiment/internal/config"
	"github.com/kleinron/uuid-insert-experiment/internal/db"
)

// LoadResolved loads config and resolves the database password.
func LoadResolved(ctx context.Context) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if err := cfg.ResolvePassword(ctx); err != nil {
		return nil, err
	}
	return cfg, nil
}

// OpenHosts opens one connection pool per target host.
func OpenHosts(cfg *config.Config, both bool, extraConns int) ([]HostDB, error) {
	targets, err := cfg.Hosts(both)
	if err != nil {
		return nil, err
	}
	var out []HostDB
	for _, t := range targets {
		sqldb, err := db.Open(cfg, t.Host, extraConns)
		if err != nil {
			for _, o := range out {
				_ = o.DB.Close()
			}
			return nil, fmt.Errorf("arm %s host %s: %w", t.Arm, t.Host, err)
		}
		out = append(out, HostDB{Arm: t.Arm, Host: t.Host, DB: sqldb})
	}
	return out, nil
}

// HostDB is an opened twin.
type HostDB struct {
	Arm  string
	Host string
	DB   *sql.DB
}

// CloseAll closes every pool.
func CloseAll(hosts []HostDB) {
	for _, h := range hosts {
		_ = h.DB.Close()
	}
}

// SignalContext cancels on SIGINT/SIGTERM.
func SignalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-ch:
			log.Printf("signal received, shutting down")
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(ch)
	}()
	return ctx, cancel
}

// BoolFlag is a tiny argv helper (no extra CLI framework).
func BoolFlag(name string) bool {
	for _, a := range os.Args[1:] {
		if a == name || a == "-"+name || a == "--"+name {
			return true
		}
	}
	return false
}

// ArgFlag returns -name=value or --name=value.
func ArgFlag(name, def string) string {
	pref := []string{"-" + name + "=", "--" + name + "="}
	for _, a := range os.Args[1:] {
		for _, p := range pref {
			if len(a) > len(p) && a[:len(p)] == p {
				return a[len(p):]
			}
		}
	}
	return def
}
