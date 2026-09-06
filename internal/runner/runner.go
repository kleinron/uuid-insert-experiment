// Package runner drives rate-limited single-row autocommit INSERTs.
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

// PKFunc returns the next primary key (16 bytes).
type PKFunc func() ([16]byte, error)

// Stats is the outcome of one Run or RunMixed.
type Stats struct {
	Inserts    int64
	Errors     int64
	Hist       *histogram.Hist
	Reads      int64
	ReadErrors int64
	ReadHist   *histogram.Hist
}

func emptyStats() *Stats {
	return &Stats{Hist: histogram.New(), ReadHist: histogram.New()}
}

// Run inserts at cfg.TargetQPS (or qps override if > 0) for dur.
// When record is false, latencies are not stored (warmup).
func Run(ctx context.Context, sqldb *sql.DB, cfg *config.Config, pk PKFunc, dur time.Duration, qps float64, record bool) (*Stats, error) {
	if qps <= 0 {
		qps = cfg.TargetQPS
	}
	if dur <= 0 {
		return emptyStats(), nil
	}

	ctx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	limiter := rate.NewLimiter(rate.Limit(qps), 1)
	var inserts, errs atomic.Int64
	hists := make([]*histogram.Hist, cfg.PoolSize)
	var wg sync.WaitGroup

	log.Printf("insert loop: qps=%.1f pool=%d duration=%s record=%v", qps, cfg.PoolSize, dur, record)
	t0 := time.Now()
	stopLog := startProgress("inserts", &inserts, &errs, t0, qps)

	for i := 0; i < cfg.PoolSize; i++ {
		hists[i] = histogram.New()
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(ctx, sqldb, pk, limiter, hists[id], record, &inserts, &errs)
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
	st := &Stats{Inserts: inserts.Load(), Errors: errs.Load(), Hist: merged, ReadHist: histogram.New()}
	elapsed := time.Since(t0).Seconds()
	actual := 0.0
	if elapsed > 0 {
		actual = float64(st.Inserts) / elapsed
	}
	log.Printf("insert loop done: inserts=%d errors=%d elapsed=%.1fs actual_qps=%.1f", st.Inserts, st.Errors, elapsed, actual)
	return st, nil
}

func worker(ctx context.Context, sqldb *sql.DB, pk PKFunc, limiter *rate.Limiter, hist *histogram.Hist, record bool, inserts, errs *atomic.Int64) {
	conn, err := sqldb.Conn(ctx)
	if err != nil {
		log.Printf("worker conn: %v", err)
		errs.Add(1)
		return
	}
	defer conn.Close()

	stmt, err := conn.PrepareContext(ctx, db.InsertSQL)
	if err != nil {
		log.Printf("worker prepare: %v", err)
		errs.Add(1)
		return
	}
	defer stmt.Close()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		if err := limiter.Wait(ctx); err != nil {
			return
		}
		id, err := pk()
		if err != nil {
			errs.Add(1)
			continue
		}
		p := datagen.PaymentRandom(rng)
		var ref any
		if p.ReferenceID != nil {
			ref = *p.ReferenceID
		}
		start := time.Now()
		_, err = stmt.ExecContext(ctx, id[:], p.MerchantID, p.CustomerID, p.Amount, p.Currency, ref)
		lat := time.Since(start)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			errs.Add(1)
			continue
		}
		inserts.Add(1)
		if record {
			hist.Record(lat)
		}
	}
}

func startProgress(label string, ops, errs *atomic.Int64, t0 time.Time, target float64) func() {
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		var last int64
		lastT := t0
		for {
			select {
			case <-stop:
				return
			case now := <-tick.C:
				n := ops.Load()
				e := errs.Load()
				dt := now.Sub(lastT).Seconds()
				inst := 0.0
				if dt > 0 {
					inst = float64(n-last) / dt
				}
				elapsed := now.Sub(t0).Seconds()
				avg := 0.0
				if elapsed > 0 {
					avg = float64(n) / elapsed
				}
				log.Printf("progress %s=%d errors=%d inst_qps=%.1f avg_qps=%.1f target=%.1f", label, n, e, inst, avg, target)
				last = n
				lastT = now
			}
		}
	}()
	return func() { close(stop); wg.Wait() }
}

// DeterministicPK returns KeyAt(n) then increments n. Used by season when rate-limited.
func DeterministicPK(start uint64) PKFunc {
	var mu sync.Mutex
	n := start
	return func() ([16]byte, error) {
		mu.Lock()
		cur := n
		n++
		mu.Unlock()
		return datagen.KeyAt(cur), nil
	}
}
