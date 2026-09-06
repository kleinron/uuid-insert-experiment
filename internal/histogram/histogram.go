// Package histogram wraps an HDR histogram of operation latencies in microseconds.
package histogram

import (
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

const (
	lowestUs  = 1
	highestUs = 60_000_000 // 60s
	sigFigs   = 3
)

// Hist is a mutex-protected HDR histogram.
type Hist struct {
	mu sync.Mutex
	h  *hdrhistogram.Histogram
}

// New records values from 1µs to 60s with 3 significant digits.
func New() *Hist {
	return &Hist{h: hdrhistogram.New(lowestUs, highestUs, sigFigs)}
}

// Record adds a duration. Values below 1µs are stored as 1.
func (h *Hist) Record(d time.Duration) {
	us := d.Microseconds()
	if us < lowestUs {
		us = lowestUs
	}
	if us > highestUs {
		us = highestUs
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = h.h.RecordValue(us)
}

// Merge adds other's recorded values into h.
func (h *Hist) Merge(other *Hist) {
	if other == nil {
		return
	}
	other.mu.Lock()
	defer other.mu.Unlock()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.h.Merge(other.h)
}

// Snapshot is percentile latency in microseconds.
type Snapshot struct {
	P50   int64 `json:"p50"`
	P95   int64 `json:"p95"`
	P99   int64 `json:"p99"`
	P999  int64 `json:"p999"`
	Max   int64 `json:"max"`
	Count int64 `json:"count"`
}

// Snapshot returns current percentiles. Empty histograms yield zeros.
func (h *Hist) Snapshot() Snapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.h.TotalCount() == 0 {
		return Snapshot{}
	}
	return Snapshot{
		P50:   h.h.ValueAtQuantile(50),
		P95:   h.h.ValueAtQuantile(95),
		P99:   h.h.ValueAtQuantile(99),
		P999:  h.h.ValueAtQuantile(99.9),
		Max:   h.h.Max(),
		Count: h.h.TotalCount(),
	}
}
