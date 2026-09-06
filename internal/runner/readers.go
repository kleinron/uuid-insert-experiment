package runner

import (
	"context"
	"database/sql"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kleinron/uuid-insert-experiment/internal/config"
	"github.com/kleinron/uuid-insert-experiment/internal/datagen"
	"github.com/kleinron/uuid-insert-experiment/internal/db"
	"github.com/kleinron/uuid-insert-experiment/internal/histogram"
	"golang.org/x/time/rate"
)

// RunMixed runs inserts at cfg.TargetQPS (or qps override if > 0) and, when
// readers are enabled, concurrent PK point lookups at cfg.ReaderQPS.
// When ReaderPoolSize is 0 the write path matches Run (write-only).
func RunMixed(ctx context.Context, sqldb *sql.DB, cfg *config.Config, pk PKFunc, dur time.Duration, qps float64, record bool) (*Stats, error) {
	if !cfg.ReadersEnabled() {
		return Run(ctx, sqldb, cfg, pk, dur, qps, record)
	}
	if dur <= 0 {
		return emptyStats(), nil
	}

	var insertSt, readSt *Stats
	var insertErr, readErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		insertSt, insertErr = Run(ctx, sqldb, cfg, pk, dur, qps, record)
	}()
	go func() {
		defer wg.Done()
		readSt, readErr = RunReads(ctx, sqldb, cfg, dur, record)
	}()
	wg.Wait()
	if insertErr != nil {
		return nil, insertErr
	}
	if readErr != nil {
		return nil, readErr
	}
	insertSt.Reads = readSt.Reads
	insertSt.ReadErrors = readSt.ReadErrors
	insertSt.ReadHist = readSt.ReadHist
	return insertSt, nil
}

// RunReads issues rate-limited SELECT ... WHERE payment_id = ? lookups against
// the deterministic preload key space. When record is false, latencies are discarded.
func RunReads(ctx context.Context, sqldb *sql.DB, cfg *config.Config, dur time.Duration, record bool) (*Stats, error) {
	if !cfg.ReadersEnabled() || dur <= 0 {
		return emptyStats(), nil
	}

	ctx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	nKeys := cfg.PreloadKeySpace()
	qps := cfg.ReaderQPS
	limiter := rate.NewLimiter(rate.Limit(qps), 1)
	var reads, errs atomic.Int64
	hists := make([]*histogram.Hist, cfg.ReaderPoolSize)
	var wg sync.WaitGroup

	log.Printf("read loop: qps=%.1f pool=%d key_space=%d duration=%s record=%v",
		qps, cfg.ReaderPoolSize, nKeys, dur, record)
	t0 := time.Now()
	stopLog := startProgress("reads", &reads, &errs, t0, qps)

	for i := 0; i < cfg.ReaderPoolSize; i++ {
		hists[i] = histogram.New()
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			readWorker(ctx, sqldb, nKeys, limiter, hists[id], record, &reads, &errs)
		}(i)
	}
	wg.Wait()
	stopLog()

	merged := histogram.New()
	if record {
		for _, h := range hists {
			merged.Merge(h)
		}
	}
	st := emptyStats()
	st.Reads = reads.Load()
	st.ReadErrors = errs.Load()
	st.ReadHist = merged
	elapsed := time.Since(t0).Seconds()
	actual := 0.0
	if elapsed > 0 {
		actual = float64(st.Reads) / elapsed
	}
	log.Printf("read loop done: reads=%d errors=%d elapsed=%.1fs actual_qps=%.1f", st.Reads, st.ReadErrors, elapsed, actual)
	return st, nil
}

func readWorker(ctx context.Context, sqldb *sql.DB, nKeys uint64, limiter *rate.Limiter, hist *histogram.Hist, record bool, reads, errs *atomic.Int64) {
	conn, err := sqldb.Conn(ctx)
	if err != nil {
		log.Printf("reader conn: %v", err)
		errs.Add(1)
		return
	}
	defer conn.Close()

	stmt, err := conn.PrepareContext(ctx, db.LookupByPKSQL)
	if err != nil {
		log.Printf("reader prepare: %v", err)
		errs.Add(1)
		return
	}
	defer stmt.Close()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var (
		merchantID, customerID, currency string
		amount                           []byte
		referenceID                      sql.NullString
	)
	for {
		if err := limiter.Wait(ctx); err != nil {
			return
		}
		id := datagen.SamplePreloadKey(rng, nKeys)
		start := time.Now()
		err = stmt.QueryRowContext(ctx, id[:]).Scan(&merchantID, &customerID, &amount, &currency, &referenceID)
		lat := time.Since(start)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			errs.Add(1)
			continue
		}
		reads.Add(1)
		if record {
			hist.Record(lat)
		}
	}
}
