// Package datagen produces fully synthetic payment rows.
//
// Non-PK columns are ASCII-ish merchant/customer ids, a DECIMAL amount,
// an ISO-4217 currency code, and an optional reference.
//
// Preload/season primary keys come from KeyAt — a shared, deterministic,
// non-UUIDv4/v7 source so both experiment twins load identical rows.
package datagen

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/rand"
)

const preloadKeyDomain = "uuid-insert-experiment/preload-key/v1"

var currencies = []string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD", "CHF", "SGD"}

const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// Payment is one synthetic row excluding the primary key.
type Payment struct {
	MerchantID  string
	CustomerID  string
	Amount      string
	Currency    string
	ReferenceID *string
}

// KeyAt returns a unique 16-byte PK for row index n.
// The bytes are a SHA-256 slice of a fixed domain and n — not UUIDv4 or UUIDv7.
// Both twins must call this with the same n sequence so preload data matches.
func KeyAt(n uint64) [16]byte {
	var seq [8]byte
	binary.BigEndian.PutUint64(seq[:], n)
	h := sha256.New()
	h.Write([]byte(preloadKeyDomain))
	h.Write(seq[:])
	sum := h.Sum(nil)
	var k [16]byte
	copy(k[:], sum[:16])
	return k
}

// SamplePreloadIndex returns a uniform index in [0, nKeys).
// nKeys should be PRELOAD_BULK_ROWS + PRELOAD_SEASON_ROWS (the locked key space).
// If nKeys is 0, the result is 0.
func SamplePreloadIndex(rng *rand.Rand, nKeys uint64) uint64 {
	if nKeys == 0 {
		return 0
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}
	if nKeys <= uint64(math.MaxInt64) {
		return uint64(rng.Int63n(int64(nKeys)))
	}
	return rng.Uint64() % nKeys
}

// SamplePreloadKey returns KeyAt of a uniform index in the preload key space.
// Measure readers use this so lookups hit already-preloaded rows, not hot insert pages.
func SamplePreloadKey(rng *rand.Rand, nKeys uint64) [16]byte {
	return KeyAt(SamplePreloadIndex(rng, nKeys))
}

// PaymentAt is a deterministic synthetic payment for row index n.
func PaymentAt(n uint64) Payment {
	const mix uint64 = 0x9e3779b97f4a7c15
	rng := rand.New(rand.NewSource(int64(n ^ mix)))
	return payment(rng)
}

// PaymentRandom returns a non-deterministic synthetic payment (measure path).
func PaymentRandom(rng *rand.Rand) Payment {
	if rng == nil {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}
	return payment(rng)
}

func payment(rng *rand.Rand) Payment {
	p := Payment{
		MerchantID: "m" + randAlnum(rng, 12+rng.Intn(8)),
		CustomerID: "c" + randAlnum(rng, 12+rng.Intn(8)),
		Currency:   currencies[rng.Intn(len(currencies))],
		Amount:     randAmount(rng),
	}
	if rng.Intn(10) < 3 {
		ref := "r" + randAlnum(rng, 10+rng.Intn(10))
		p.ReferenceID = &ref
	}
	return p
}

func randAlnum(rng *rand.Rand, n int) string {
	if n > 29 {
		n = 29
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = idAlphabet[rng.Intn(len(idAlphabet))]
	}
	return string(b)
}

func randAmount(rng *rand.Rand) string {
	// DECIMAL(12,2): keep values in a realistic payments band.
	cents := 1 + rng.Intn(5_000_000) // 0.01 .. 50,000.00
	return formatCents(cents)
}

func formatCents(cents int) string {
	neg := ""
	if cents < 0 {
		neg = "-"
		cents = -cents
	}
	whole := cents / 100
	frac := cents % 100
	return neg + itoa(whole) + "." + pad2(frac)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func pad2(n int) string {
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}
