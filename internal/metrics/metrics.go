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

// Result is one warmup-discarded, measured insert run.
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
	LatencyUS       histogram.Snapshot `json:"latency_us"`
}

// Write stores a timestamped JSON file and a {arm}-latest.json copy.
func Write(dir string, r Result) (string, error) {
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

// FprintTable writes a compact comparison table.
func FprintTable(w io.Writer, results []Result) {
	if len(results) == 0 {
		fmt.Fprintln(w, "no result JSON files in results/")
		return
	}
	fmt.Fprintf(w, "%-4s  %-20s  %8s  %8s  %8s  %8s  %8s  %8s  %8s\n",
		"arm", "started", "qps", "n", "err", "p50µs", "p95µs", "p99µs", "p999µs")
	for _, r := range results {
		fmt.Fprintf(w, "%-4s  %-20s  %8.1f  %8d  %8d  %8d  %8d  %8d  %8d\n",
			r.Arm, r.StartedAt.UTC().Format("2006-01-02T15:04:05Z"),
			r.ActualQPS, r.Inserts, r.Errors,
			r.LatencyUS.P50, r.LatencyUS.P95, r.LatencyUS.P99, r.LatencyUS.P999)
	}
}
