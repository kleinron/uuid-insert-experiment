package db

import "testing"

func TestSplitSQL(t *testing.T) {
	script := `-- comment
CREATE TABLE IF NOT EXISTS t (id INT);
-- another
INSERT INTO t VALUES (1);
`
	got := splitSQL(script)
	if len(got) != 2 {
		t.Fatalf("got %d statements: %#v", len(got), got)
	}
	if got[0] != "CREATE TABLE IF NOT EXISTS t (id INT)" {
		t.Fatalf("stmt0=%q", got[0])
	}
}

func TestMultiInsertSQL(t *testing.T) {
	s := MultiInsertSQL(2)
	want := "INSERT INTO payments (payment_id, merchant_id, customer_id, amount, currency, reference_id) VALUES (?,?,?,?,?,?),(?,?,?,?,?,?)"
	if s != want {
		t.Fatalf("got %s", s)
	}
}
