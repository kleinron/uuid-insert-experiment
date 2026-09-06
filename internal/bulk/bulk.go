// Package bulk inserts deterministic preload rows (shared key source, both twins).
package bulk

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kleinron/uuid-insert-experiment/internal/config"
	"github.com/kleinron/uuid-insert-experiment/internal/datagen"
	"github.com/kleinron/uuid-insert-experiment/internal/db"
)

// InsertRange writes rows [start, start+count) using PRELOAD_MODE.
func InsertRange(ctx context.Context, sqldb *sql.DB, cfg *config.Config, start, count uint64) error {
	if count == 0 {
		return nil
	}
	switch cfg.PreloadMode {
	case "loaddata":
		return loadData(ctx, sqldb, cfg, start, count)
	default:
		return batchInsert(ctx, sqldb, cfg, start, count)
	}
}

func batchInsert(ctx context.Context, sqldb *sql.DB, cfg *config.Config, start, count uint64) error {
	workers := cfg.PreloadWorkers
	if uint64(workers) > count {
		workers = int(count)
	}
	batch := cfg.PreloadBatchSize
	var done atomic.Uint64
	var firstErr error
	var errOnce sync.Once
	setErr := func(err error) {
		errOnce.Do(func() { firstErr = err })
	}

	chunk := count / uint64(workers)
	var wg sync.WaitGroup
	log.Printf("bulk batch insert: rows=%d start=%d workers=%d batch=%d", count, start, workers, batch)
	t0 := time.Now()
	stopLog := startProgress(&done, count, t0)

	for w := 0; w < workers; w++ {
		w := w
		lo := start + uint64(w)*chunk
		hi := lo + chunk
		if w == workers-1 {
			hi = start + count
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := insertSpan(ctx, sqldb, batch, lo, hi, &done); err != nil {
				setErr(err)
			}
		}()
	}
	wg.Wait()
	stopLog()
	if firstErr != nil {
		return firstErr
	}
	log.Printf("bulk batch insert complete: rows=%d elapsed=%s", count, time.Since(t0).Truncate(time.Millisecond))
	return nil
}

func insertSpan(ctx context.Context, sqldb *sql.DB, batch int, lo, hi uint64, done *atomic.Uint64) error {
	n := hi - lo
	if n == 0 {
		return nil
	}
	fullSQL := db.MultiInsertSQL(batch)
	stmt, err := sqldb.PrepareContext(ctx, fullSQL)
	if err != nil {
		return fmt.Errorf("prepare batch %d: %w", batch, err)
	}
	defer stmt.Close()

	args := make([]any, 0, batch*6)
	var tail *sql.Stmt
	var tailN int

	flush := func(s *sql.Stmt, a []any, rows int) error {
		if _, err := s.ExecContext(ctx, a...); err != nil {
			return err
		}
		done.Add(uint64(rows))
		return nil
	}

	i := lo
	for i+uint64(batch) <= hi {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		args = args[:0]
		for k := uint64(0); k < uint64(batch); k++ {
			args = appendPayment(args, i+k)
		}
		if err := flush(stmt, args, batch); err != nil {
			return fmt.Errorf("insert [%d,%d): %w", i, i+uint64(batch), err)
		}
		i += uint64(batch)
	}
	if i < hi {
		remain := int(hi - i)
		if tail == nil || tailN != remain {
			if tail != nil {
				_ = tail.Close()
			}
			tail, err = sqldb.PrepareContext(ctx, db.MultiInsertSQL(remain))
			if err != nil {
				return err
			}
			tailN = remain
			defer tail.Close()
		}
		args = args[:0]
		for k := uint64(0); k < uint64(remain); k++ {
			args = appendPayment(args, i+k)
		}
		if err := flush(tail, args, remain); err != nil {
			return fmt.Errorf("insert tail [%d,%d): %w", i, hi, err)
		}
	}
	return nil
}

func appendPayment(args []any, n uint64) []any {
	pk := datagen.KeyAt(n)
	p := datagen.PaymentAt(n)
	var ref any
	if p.ReferenceID != nil {
		ref = *p.ReferenceID
	}
	return append(args, pk[:], p.MerchantID, p.CustomerID, p.Amount, p.Currency, ref)
}

func loadData(ctx context.Context, sqldb *sql.DB, cfg *config.Config, start, count uint64) error {
	const chunk = uint64(500_000)
	tmp, err := os.MkdirTemp("", "uuid-insert-preload-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	log.Printf("bulk LOAD DATA: rows=%d start=%d chunk=%d", count, start, chunk)
	t0 := time.Now()
	var loaded uint64
	for off := uint64(0); off < count; off += chunk {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n := chunk
		if off+n > count {
			n = count - off
		}
		path := filepath.Join(tmp, fmt.Sprintf("chunk-%d.tsv", off))
		if err := writeTSV(path, start+off, n); err != nil {
			return err
		}
		q := fmt.Sprintf(`LOAD DATA LOCAL INFILE '%s' INTO TABLE payments
			FIELDS TERMINATED BY '\t' OPTIONALLY ENCLOSED BY '"'
			LINES TERMINATED BY '\n'
			(@pid, merchant_id, customer_id, amount, currency, @ref)
			SET payment_id = UNHEX(@pid),
			    reference_id = NULLIF(@ref, '')`, filepath.ToSlash(path))
		if _, err := sqldb.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("LOAD DATA LOCAL (need local_infile=1 on server and PRELOAD_MODE=loaddata): %w", err)
		}
		loaded += n
		_ = os.Remove(path)
		log.Printf("LOAD DATA progress %d/%d (%.1f%%) elapsed=%s", loaded, count, 100*float64(loaded)/float64(count), time.Since(t0).Truncate(time.Second))
	}
	log.Printf("bulk LOAD DATA complete: rows=%d elapsed=%s", count, time.Since(t0).Truncate(time.Millisecond))
	return nil
}

func writeTSV(path string, start, count uint64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	for n := start; n < start+count; n++ {
		pk := datagen.KeyAt(n)
		p := datagen.PaymentAt(n)
		ref := ""
		if p.ReferenceID != nil {
			ref = *p.ReferenceID
		}
		if _, err := fmt.Fprintf(f, "%s\t%s\t%s\t%s\t%s\t%s\n",
			hex.EncodeToString(pk[:]), p.MerchantID, p.CustomerID, p.Amount, p.Currency, ref); err != nil {
			_ = f.Close()
			return err
		}
	}
	return f.Close()
}

func startProgress(done *atomic.Uint64, total uint64, t0 time.Time) func() {
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				n := done.Load()
				elapsed := time.Since(t0).Seconds()
				rps := 0.0
				if elapsed > 0 {
					rps = float64(n) / elapsed
				}
				log.Printf("preload progress %d/%d (%.1f%%) ~%.0f rows/s", n, total, 100*float64(n)/float64(total), rps)
			}
		}
	}()
	return func() { close(stop); wg.Wait() }
}
