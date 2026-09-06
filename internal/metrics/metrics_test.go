package metrics

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kleinron/uuid-insert-experiment/internal/histogram"
)

func TestWriteAndLoad(t *testing.T) {
	dir := t.TempDir()
	r := Result{
		Arm:             "v4",
		Host:            "localhost",
		StartedAt:       time.Date(2026, 9, 6, 13, 0, 0, 0, time.UTC),
		EndedAt:         time.Date(2026, 9, 6, 13, 20, 0, 0, time.UTC),
		DurationSeconds: 1200,
		TargetQPS:       500,
		ActualQPS:       499.5,
		Inserts:         599400,
		PoolSize:        32,
		LatencyUS:       histogram.Snapshot{P50: 800, P95: 2000, P99: 4000, P999: 9000},
	}
	path, err := Write(dir, r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "v4-latest.json")); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Arm != "v4" || got[0].Inserts != 599400 {
		t.Fatalf("loaded %+v", got)
	}
	var buf bytes.Buffer
	FprintTable(&buf, got)
	if !bytes.Contains(buf.Bytes(), []byte("v4")) {
		t.Fatalf("table: %s", buf.String())
	}
}
