package runner

import (
	"context"
	"testing"
	"time"

	"github.com/kleinron/uuid-insert-experiment/internal/config"
)

func TestDeterministicPKSequence(t *testing.T) {
	pk := DeterministicPK(10)
	a, err := pk()
	if err != nil {
		t.Fatal(err)
	}
	b, err := pk()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("expected distinct keys")
	}
	again := DeterministicPK(10)
	c, _ := again()
	if c != a {
		t.Fatal("DeterministicPK(10) first value not stable")
	}
}

func TestRunMixedReadersDisabledNoDuration(t *testing.T) {
	cfg := &config.Config{TargetQPS: 10, PoolSize: 1, ReaderPoolSize: 0}
	st, err := RunMixed(context.Background(), nil, cfg, nil, 0, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Inserts != 0 || st.Reads != 0 || st.Hist == nil || st.ReadHist == nil {
		t.Fatalf("unexpected stats %+v", st)
	}
}

func TestRunReadsDisabled(t *testing.T) {
	cfg := &config.Config{ReaderPoolSize: 0, ReaderQPS: 500, PreloadBulkRows: 10}
	st, err := RunReads(context.Background(), nil, cfg, time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Reads != 0 {
		t.Fatalf("disabled readers should do no work, reads=%d", st.Reads)
	}
}
