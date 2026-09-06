package datagen

import (
	"math/rand"
	"testing"
	"unicode"
)

func TestKeyAtDeterministicAndUnique(t *testing.T) {
	a := KeyAt(0)
	b := KeyAt(0)
	if a != b {
		t.Fatal("KeyAt(0) is not deterministic")
	}
	seen := make(map[[16]byte]uint64, 10_000)
	for n := uint64(0); n < 10_000; n++ {
		k := KeyAt(n)
		if prev, ok := seen[k]; ok {
			t.Fatalf("duplicate key for n=%d and n=%d", prev, n)
		}
		seen[k] = n
	}
	if KeyAt(1) == KeyAt(2) {
		t.Fatal("KeyAt(1) == KeyAt(2)")
	}
}

func TestKeyAtNotRFCVersion4Or7(t *testing.T) {
	// Hash-derived keys must not be used as the v4/v7 A/B. Spot-check that we
	// do not systematically stamp RFC version 4 or 7 (random collisions ok).
	v4, v7 := 0, 0
	const n = 1000
	for i := uint64(0); i < n; i++ {
		k := KeyAt(i)
		switch k[6] >> 4 {
		case 4:
			v4++
		case 7:
			v7++
		}
	}
	if v4 == n || v7 == n {
		t.Fatalf("preload keys look like a single UUID version (v4=%d v7=%d)", v4, v7)
	}
}

func TestSamplePreloadIndexRange(t *testing.T) {
	if SamplePreloadIndex(nil, 0) != 0 {
		t.Fatal("nKeys=0 should return 0")
	}
	rng := rand.New(rand.NewSource(1))
	const nKeys uint64 = 17
	seen := make(map[uint64]int)
	for i := 0; i < 2000; i++ {
		idx := SamplePreloadIndex(rng, nKeys)
		if idx >= nKeys {
			t.Fatalf("index %d out of [0, %d)", idx, nKeys)
		}
		seen[idx]++
	}
	if len(seen) < 10 {
		t.Fatalf("expected broad coverage of [0,%d), got %d distinct", nKeys, len(seen))
	}
	if SamplePreloadIndex(rand.New(rand.NewSource(2)), 1) != 0 {
		t.Fatal("nKeys=1 should always return 0")
	}
}

func TestSamplePreloadKeyUsesKeyAt(t *testing.T) {
	nKeys := uint64(200)
	for i := 0; i < 50; i++ {
		k := SamplePreloadKey(rand.New(rand.NewSource(100+int64(i))), nKeys)
		found := false
		for n := uint64(0); n < nKeys; n++ {
			if KeyAt(n) == k {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("sampled key not in preload space (draw %d)", i)
		}
	}
}

func TestSamplePreloadKeyDeterministicWithSeed(t *testing.T) {
	a := SamplePreloadKey(rand.New(rand.NewSource(7)), 500)
	b := SamplePreloadKey(rand.New(rand.NewSource(7)), 500)
	if a != b {
		t.Fatal("same seed should yield the same first sample")
	}
	if a != KeyAt(SamplePreloadIndex(rand.New(rand.NewSource(7)), 500)) {
		t.Fatal("SamplePreloadKey must be KeyAt(SamplePreloadIndex)")
	}
}

func TestPaymentAtDeterministic(t *testing.T) {
	a := PaymentAt(42)
	b := PaymentAt(42)
	if a.MerchantID != b.MerchantID || a.CustomerID != b.CustomerID || a.Amount != b.Amount || a.Currency != b.Currency {
		t.Fatalf("PaymentAt(42) not deterministic: %+v vs %+v", a, b)
	}
	if (a.ReferenceID == nil) != (b.ReferenceID == nil) {
		t.Fatal("reference pointer mismatch")
	}
	if a.ReferenceID != nil && *a.ReferenceID != *b.ReferenceID {
		t.Fatal("reference value mismatch")
	}
	c := PaymentAt(43)
	if c.MerchantID == a.MerchantID && c.CustomerID == a.CustomerID && c.Amount == a.Amount {
		t.Fatal("PaymentAt(43) unexpectedly identical to PaymentAt(42)")
	}
}

func TestPaymentFieldConstraints(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		p := PaymentRandom(rng)
		checkPayment(t, p)
	}
	checkPayment(t, PaymentAt(0))
	checkPayment(t, PaymentAt(1_000_000))
}

func checkPayment(t *testing.T, p Payment) {
	t.Helper()
	if len(p.MerchantID) == 0 || len(p.MerchantID) > 30 {
		t.Fatalf("merchant_id length %d", len(p.MerchantID))
	}
	if len(p.CustomerID) == 0 || len(p.CustomerID) > 30 {
		t.Fatalf("customer_id length %d", len(p.CustomerID))
	}
	if len(p.Currency) != 3 {
		t.Fatalf("currency %q", p.Currency)
	}
	for _, r := range p.MerchantID + p.CustomerID {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			t.Fatalf("non ASCII-ish id rune %q", r)
		}
	}
	if p.Amount == "" {
		t.Fatal("empty amount")
	}
	if p.ReferenceID != nil && len(*p.ReferenceID) > 30 {
		t.Fatalf("reference_id length %d", len(*p.ReferenceID))
	}
}
