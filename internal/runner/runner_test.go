package runner

import "testing"

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
