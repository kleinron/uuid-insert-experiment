package histogram

import (
	"testing"
	"time"
)

func TestPercentiles(t *testing.T) {
	h := New()
	for i := 1; i <= 1000; i++ {
		h.Record(time.Duration(i) * time.Microsecond)
	}
	s := h.Snapshot()
	if s.Count != 1000 {
		t.Fatalf("count=%d", s.Count)
	}
	if s.P50 < 400 || s.P50 > 600 {
		t.Fatalf("p50=%d want ~500", s.P50)
	}
	if s.P95 < 900 || s.P95 > 980 {
		t.Fatalf("p95=%d want ~950", s.P95)
	}
	if s.P99 < 970 || s.P99 > 1000 {
		t.Fatalf("p99=%d want ~990", s.P99)
	}
	if s.Max != 1000 {
		t.Fatalf("max=%d", s.Max)
	}
}

func TestEmptySnapshot(t *testing.T) {
	s := New().Snapshot()
	if s.Count != 0 || s.P50 != 0 {
		t.Fatalf("empty snapshot %+v", s)
	}
}

func TestMerge(t *testing.T) {
	a := New()
	b := New()
	a.Record(10 * time.Microsecond)
	b.Record(20 * time.Microsecond)
	a.Merge(b)
	s := a.Snapshot()
	if s.Count != 2 {
		t.Fatalf("merged count=%d", s.Count)
	}
	if s.Max != 20 {
		t.Fatalf("merged max=%d", s.Max)
	}
}
