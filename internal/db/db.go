// Package db opens MySQL connections and runs schema / tablespace helpers.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/kleinron/uuid-insert-experiment/internal/config"
)

const InsertSQL = `INSERT INTO payments (payment_id, merchant_id, customer_id, amount, currency, reference_id) VALUES (?, ?, ?, ?, ?, ?)`

// LookupByPKSQL is a clustered primary-key point lookup (no range scan).
const LookupByPKSQL = `SELECT merchant_id, customer_id, amount, currency, reference_id FROM payments WHERE payment_id = ?`

// Open returns a pinged pool sized for the workload.
func Open(cfg *config.Config, host string, extraConns int) (*sql.DB, error) {
	dsn, err := DSN(cfg, host)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	n := cfg.PoolSize
	if extraConns > n {
		n = extraConns
	}
	db.SetMaxOpenConns(n)
	db.SetMaxIdleConns(n)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %s: %w", host, err)
	}
	return db, nil
}

// DSN builds a go-sql-driver DSN. allowAllFiles is enabled for LOAD DATA LOCAL.
func DSN(cfg *config.Config, host string) (string, error) {
	if host == "" {
		return "", fmt.Errorf("empty mysql host")
	}
	mc := mysql.NewConfig()
	mc.User = cfg.User
	mc.Passwd = cfg.Password
	mc.Net = "tcp"
	mc.Addr = net.JoinHostPort(host, strconv.Itoa(cfg.Port))
	mc.DBName = cfg.Database
	mc.Params = map[string]string{"charset": "utf8mb4"}
	mc.Timeout = 30 * time.Second
	mc.ReadTimeout = 120 * time.Second
	mc.WriteTimeout = 120 * time.Second
	mc.AllowNativePasswords = true
	mc.ParseTime = true
	mc.AllowAllFiles = cfg.PreloadMode == "loaddata"
	if cfg.TLS {
		// Same-VPC RDS: encrypt in transit without requiring the Amazon CA on the client.
		mc.TLSConfig = "skip-verify"
	}
	return mc.FormatDSN(), nil
}

// ApplySchemaFile executes the statements in path against db.
func ApplySchemaFile(ctx context.Context, sqldb *sql.DB, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema %s: %w", path, err)
	}
	return ApplySchema(ctx, sqldb, string(raw))
}

// ApplySchema executes semicolon-separated SQL, skipping comments-only chunks.
func ApplySchema(ctx context.Context, sqldb *sql.DB, script string) error {
	for _, stmt := range splitSQL(script) {
		if _, err := sqldb.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("schema exec: %w", err)
		}
	}
	return nil
}

func splitSQL(script string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(script, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, stmt := range strings.Split(b.String(), ";") {
		s := strings.TrimSpace(stmt)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// TablespaceRevisitBytes is the locked gate: measured data+index ≥ 50 GiB
// means stop and revisit instance class / InnoDB buffer pool (BP).
const TablespaceRevisitBytes int64 = 50 * 1024 * 1024 * 1024

// Report is a tablespace / table size snapshot.
type Report struct {
	TableName    string `json:"table_name"`
	TableSchema  string `json:"table_schema"`
	TableRows    int64  `json:"table_rows"`
	DataLength   int64  `json:"data_length"`
	IndexLength  int64  `json:"index_length"`
	DataFree     int64  `json:"data_free"`
	AvgRowLength int64  `json:"avg_row_length"`
	Engine       string `json:"engine"`
}

// Tablespace reads information_schema.TABLES for payments.
func Tablespace(ctx context.Context, sqldb *sql.DB, schema, table string) (Report, error) {
	var r Report
	err := sqldb.QueryRowContext(ctx, `
		SELECT TABLE_SCHEMA, TABLE_NAME, ENGINE,
		       COALESCE(TABLE_ROWS, 0), COALESCE(DATA_LENGTH, 0),
		       COALESCE(INDEX_LENGTH, 0), COALESCE(DATA_FREE, 0),
		       COALESCE(AVG_ROW_LENGTH, 0)
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, schema, table).Scan(
		&r.TableSchema, &r.TableName, &r.Engine,
		&r.TableRows, &r.DataLength, &r.IndexLength, &r.DataFree, &r.AvgRowLength,
	)
	if err != nil {
		return r, fmt.Errorf("information_schema.TABLES: %w", err)
	}
	return r, nil
}

// DataAndIndexBytes is DATA_LENGTH + INDEX_LENGTH (InnoDB clustered PK is in data_length).
func (r Report) DataAndIndexBytes() int64 {
	return r.DataLength + r.IndexLength
}

// OverRevisitThreshold reports whether measured tablespace is ≥ 50 GiB.
func (r Report) OverRevisitThreshold() bool {
	return r.DataAndIndexBytes() >= TablespaceRevisitBytes
}

// MultiInsertSQL returns a multi-row INSERT with n value tuples.
func MultiInsertSQL(n int) string {
	if n < 1 {
		n = 1
	}
	var b strings.Builder
	b.Grow(80 + n*14)
	b.WriteString("INSERT INTO payments (payment_id, merchant_id, customer_id, amount, currency, reference_id) VALUES ")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("(?,?,?,?,?,?)")
	}
	return b.String()
}
