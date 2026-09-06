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
		ReaderPoolSize:  8,
		ReaderQPS:       500,
		ActualReadQPS:   498.0,
		Reads:           597600,
		ReadLatencyUS:   histogram.Snapshot{P50: 120, P95: 400, P99: 900, P999: 2500},
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
	if got[0].Reads != 597600 || got[0].ReaderPoolSize != 8 {
		t.Fatalf("loaded reads %+v", got[0])
	}
	if len(got[0].Ops) != 2 || got[0].Ops[0].Op != "insert" || got[0].Ops[1].Op != "read" {
		t.Fatalf("ops %+v", got[0].Ops)
	}
	var buf bytes.Buffer
	FprintTable(&buf, got)
	if !bytes.Contains(buf.Bytes(), []byte("v4")) || !bytes.Contains(buf.Bytes(), []byte("insert")) || !bytes.Contains(buf.Bytes(), []byte("read")) {
		t.Fatalf("table: %s", buf.String())
	}
}

func TestLoadDirOldInsertOnlyJSON(t *testing.T) {
	dir := t.TempDir()
	old := `{
  "arm": "v4",
  "started_at": "2026-09-06T13:00:00Z",
  "inserts": 100,
  "actual_qps": 99.5,
  "latency_us": {"p50": 10, "p95": 20, "p99": 30, "p999": 40}
}`
	if err := os.WriteFile(filepath.Join(dir, "v4-20260906T130000Z.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Inserts != 100 || got[0].Reads != 0 {
		t.Fatalf("loaded %+v", got)
	}
	var buf bytes.Buffer
	FprintTable(&buf, got)
	if bytes.Contains(buf.Bytes(), []byte("read")) {
		t.Fatalf("legacy JSON should not list op=read: %s", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("insert")) {
		t.Fatalf("table: %s", buf.String())
	}
}

func TestWriteEnsuresInsertOpOnlyWhenReadersOff(t *testing.T) {
	dir := t.TempDir()
	r := Result{
		Arm:       "v7",
		StartedAt: time.Date(2026, 9, 6, 14, 0, 0, 0, time.UTC),
		Inserts:   10,
		LatencyUS: histogram.Snapshot{P50: 1},
	}
	if _, err := Write(dir, r); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Ops) != 1 || got[0].Ops[0].Op != "insert" {
		t.Fatalf("ops %+v", got[0].Ops)
	}
	var buf bytes.Buffer
	FprintTable(&buf, got)
	if bytes.Contains(buf.Bytes(), []byte("read")) {
		t.Fatalf("write-only table should not list op=read: %s", buf.String())
	}
}
