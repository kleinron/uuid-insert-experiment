// Package metrics writes and summarizes measure-run results under results/.
package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kleinron/uuid-insert-experiment/internal/histogram"
)

// OpMetrics is one labeled operation (insert or read) in a measure run.
type OpMetrics struct {
	Op        string             `json:"op"`
	TargetQPS float64            `json:"target_qps"`
	ActualQPS float64            `json:"actual_qps"`
	Count     int64              `json:"count"`
	Errors    int64              `json:"errors"`
	PoolSize  int                `json:"pool_size"`
	LatencyUS histogram.Snapshot `json:"latency_us"`
}

// Result is one warmup-discarded, measured run (inserts, plus optional PK lookups).
type Result struct {
	Arm             string             `json:"arm"`
	Host            string             `json:"host"`
	StartedAt       time.Time          `json:"started_at"`
	EndedAt         time.Time          `json:"ended_at"`
	DurationSeconds float64            `json:"duration_s"`
	TargetQPS       float64            `json:"target_qps"`
	ActualQPS       float64            `json:"actual_qps"`
	Inserts         int64              `json:"inserts"`
	Errors          int64              `json:"errors"`
	PoolSize        int                `json:"pool_size"`
	LatencyUS       histogram.Snapshot `json:"latency_us"` // insert latency (backward compatible)
	ReaderPoolSize  int                `json:"reader_pool_size"`
	ReaderQPS       float64            `json:"reader_qps"`
	ActualReadQPS   float64            `json:"actual_read_qps"`
	Reads           int64              `json:"reads"`
	ReadErrors      int64              `json:"read_errors"`
	ReadLatencyUS   histogram.Snapshot `json:"read_latency_us"`
	Ops             []OpMetrics        `json:"ops"`
}

// Write stores a timestamped JSON file and a {arm}-latest.json copy.
func Write(dir string, r Result) (string, error) {
	r.ensureOps()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stamp := r.StartedAt.UTC().Format("20060102T150405Z")
	name := fmt.Sprintf("%s-%s.json", r.Arm, stamp)
	path := filepath.Join(dir, name)
	if err := writeJSON(path, r); err != nil {
		return "", err
	}
	latest := filepath.Join(dir, r.Arm+"-latest.json")
	if err := writeJSON(latest, r); err != nil {
		return path, err
	}
	return path, nil
}

func (r *Result) ensureOps() {
	if len(r.Ops) > 0 {
		return
	}
	r.Ops = []OpMetrics{{
		Op:        "insert",
		TargetQPS: r.TargetQPS,
		ActualQPS: r.ActualQPS,
		Count:     r.Inserts,
		Errors:    r.Errors,
		PoolSize:  r.PoolSize,
		LatencyUS: r.LatencyUS,
	}}
	if r.ReaderPoolSize > 0 || r.Reads > 0 {
		r.Ops = append(r.Ops, OpMetrics{
			Op:        "read",
			TargetQPS: r.ReaderQPS,
			ActualQPS: r.ActualReadQPS,
			Count:     r.Reads,
			Errors:    r.ReadErrors,
			PoolSize:  r.ReaderPoolSize,
			LatencyUS: r.ReadLatencyUS,
		})
	}
}

func writeJSON(path string, r Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// LoadDir reads all non-latest result JSON files.
func LoadDir(dir string) ([]Result, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Result
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if strings.HasSuffix(e.Name(), "-latest.json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r Result
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out, nil
}

// FprintTable writes a compact comparison table (one row per arm/op).
func FprintTable(w io.Writer, results []Result) {
	if len(results) == 0 {
		fmt.Fprintln(w, "no result JSON files in results/")
		return
	}
	fmt.Fprintf(w, "%-4s  %-6s  %-20s  %8s  %8s  %8s  %8s  %8s  %8s  %8s\n",
		"arm", "op", "started", "qps", "n", "err", "p50µs", "p95µs", "p99µs", "p999µs")
	for _, r := range results {
		fprintOp(w, r, "insert", r.ActualQPS, r.Inserts, r.Errors, r.LatencyUS)
		if r.ReaderPoolSize > 0 || r.Reads > 0 {
			fprintOp(w, r, "read", r.ActualReadQPS, r.Reads, r.ReadErrors, r.ReadLatencyUS)
		}
	}
}

func fprintOp(w io.Writer, r Result, op string, qps float64, n, errs int64, lat histogram.Snapshot) {
	fmt.Fprintf(w, "%-4s  %-6s  %-20s  %8.1f  %8d  %8d  %8d  %8d  %8d  %8d\n",
		r.Arm, op, r.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
		qps, n, errs, lat.P50, lat.P95, lat.P99, lat.P999)
}
