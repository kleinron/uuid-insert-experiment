package db

import (
	"strings"
	"testing"

	"github.com/kleinron/uuid-insert-experiment/internal/config"
)

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

func TestDSN_TLSDefaultOff(t *testing.T) {
	cfg := &config.Config{User: "exp_app", Password: "x", Port: 3306, Database: "payments_exp"}
	dsn, err := DSN(cfg, "v4.example.internal")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dsn, "tls=") {
		t.Fatalf("default DSN should not set tls: %s", dsn)
	}
}

func TestDSN_TLSSkipVerify(t *testing.T) {
	cfg := &config.Config{User: "exp_app", Password: "x", Port: 3306, Database: "payments_exp", TLS: true}
	dsn, err := DSN(cfg, "v4.example.internal")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "tls=skip-verify") {
		t.Fatalf("MYSQL_TLS=true should use skip-verify for same-VPC RDS, got %s", dsn)
	}
}

func TestMultiInsertSQL(t *testing.T) {
	s := MultiInsertSQL(2)
	want := "INSERT INTO payments (payment_id, merchant_id, customer_id, amount, currency, reference_id) VALUES (?,?,?,?,?,?),(?,?,?,?,?,?)"
	if s != want {
		t.Fatalf("got %s", s)
	}
}
