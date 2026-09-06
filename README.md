# uuid-insert-experiment

Rerunnable MySQL UUID insert load harness in Go. Compares **UUIDv4 vs UUIDv7** stored as `BINARY(16)` InnoDB primary keys at about **500 inserts/sec**.

This repo is **app / preload / measure code only**. Twin RDS instances are separate infrastructure.

All row data is **synthetic**. There are no production credentials in the tree — use placeholders in `.env.example` and your own secrets locally or via AWS Secrets Manager.

## Rerun

```
config → schema → preload → warmup → measure → export
```

1. Copy `.env.example` to `.env` and fill real twin hosts / secret ARN (or a local `MYSQL_PASSWORD`).
2. `make config-check` — prints settings, resolves the password, pings MySQL.
3. `make schema` (or `make schema BOTH=1`) — applies `sql/001_schema.sql`.
4. `make preload` — bulk load, then identical seasoning on the current twin. Repeat for the other twin, or pass `BOTH=1`. Use the **same** `PRELOAD_*_ROWS` on both sides.
5. `EXPERIMENT_ARM=v4 make run-arm` — warmup (histograms discarded + client settle), measure, export.
6. `EXPERIMENT_ARM=v7 make run-arm` — same protocol on the v7 twin.
7. `make export-metrics` — HDR p50 / p95 / p99 / p999 table from `results/`.
8. `make tablespace-report` — `information_schema` size snapshot.

```bash
cp .env.example .env
# edit .env

make config-check
make schema BOTH=1
make preload BOTH=1          # full size is ~185M + ~15M; override for tests
EXPERIMENT_ARM=v4 make run-arm
EXPERIMENT_ARM=v7 make run-arm
make export-metrics
```

`make run-arm` is `warmup` → `measure` → `export-metrics`.

## Small-scale path (no AWS)

Unit tests and binaries do not need MySQL or AWS:

```bash
make test
make build
```

For a real insert smoke run, point both hosts at one local MySQL and set `MYSQL_PASSWORD` so Secrets Manager is skipped:

```bash
docker run --name payments-exp-mysql \
  -e MYSQL_ROOT_PASSWORD=dev \
  -e MYSQL_DATABASE=payments_exp \
  -e MYSQL_USER=exp_app \
  -e MYSQL_PASSWORD=dev \
  -p 3306:3306 -d mysql:8.0 --local-infile=1

export MYSQL_HOST_V4=127.0.0.1 MYSQL_HOST_V7=127.0.0.1
export MYSQL_PASSWORD=dev
export EXPERIMENT_ARM=v4
export PRELOAD_BULK_ROWS=1000 PRELOAD_SEASON_ROWS=100
export WARMUP_MINUTES=0.05 MEASURE_MINUTES=0.1 SETTLE_SECONDS=0
export TARGET_QPS=50 POOL_SIZE=4

make config-check
make schema
make preload
make warmup
make measure
make export-metrics
```

Or: `MYSQL_PASSWORD=dev ./scripts/local-smoke.sh`

Do not pass `BOTH=1` when `MYSQL_HOST_V4` and `MYSQL_HOST_V7` are the same instance — preload keys would collide.

`WARMUP_MINUTES` and `MEASURE_MINUTES` accept fractional minutes (0.05 ≈ 3s).

What you need for that path:

| Need | Why |
| --- | --- |
| Go 1.22+ | build / `go run` |
| MySQL 8 reachable | schema, preload, loadgen |
| `MYSQL_PASSWORD` | skips AWS |
| Optional Docker | easiest local server |

What you need for the full twin-RDS experiment (not in this repo): two similar RDS MySQL instances, a Secrets Manager secret whose JSON includes `password`, and AWS credentials that can `GetSecretValue`.

## Schema

`sql/001_schema.sql` — PK only, no secondary indexes:

```sql
CREATE TABLE payments (
  payment_id   BINARY(16) PRIMARY KEY,
  merchant_id  VARCHAR(30) NOT NULL,
  customer_id  VARCHAR(30) NOT NULL,
  amount       DECIMAL(12, 2) NOT NULL,
  currency     CHAR(3) NOT NULL,
  reference_id VARCHAR(30)
);
```

## Keys and synthetic data

| Phase | Primary key | Non-PK columns |
| --- | --- | --- |
| Preload bulk + season | `datagen.KeyAt(n)` — SHA-256 of a fixed domain + row index, 16 bytes. **Not** UUIDv4/v7. Identical on both twins for the same `n`. | `datagen.PaymentAt(n)` (deterministic) |
| Warmup + measure | `internal/uuidx` UUIDv4 or UUIDv7 → `[16]byte`, chosen by `EXPERIMENT_ARM` | random synthetic payment |

Bulk rows use `n = 0 .. PRELOAD_BULK_ROWS-1`. Season continues at `n = PRELOAD_BULK_ROWS`. Do not mix arm-typed UUIDs into preload.

## Measure protocol

- `TARGET_QPS=500`, `POOL_SIZE=32`
- Single-row prepared `INSERT`, autocommit
- Warmup `WARMUP_MINUTES=25` (20–30) at full rate; histograms discarded
- ~2 minutes client settle at the end of warmup (`SETTLE_SECONDS=120`)
- Measure `MEASURE_MINUTES=20`; HDR p50/p95/p99/p999 → `results/{arm}-{timestamp}.json` and `results/{arm}-latest.json`
- `EXPERIMENT_ARM=v4|v7` selects `MYSQL_HOST_V4` or `MYSQL_HOST_V7` **and** the UUID generator

## Preload (hybrid)

1. **Bulk** (`make preload-bulk`) — multi-row `INSERT` batches by default (`PRELOAD_MODE=batch`). Optional `PRELOAD_MODE=loaddata` uses `LOAD DATA LOCAL INFILE` (server must allow `local_infile`).
2. **Season** (`make preload-season`) — same shared key source. Default is also batched so both twins stay identical and 10–20M rows stay practical. `SEASON_QPS>0` switches to rate-limited single-row inserts.

Override counts with `PRELOAD_BULK_ROWS` / `PRELOAD_SEASON_ROWS` for tests.

## Config

Copied from `.env.example`. Commands call `godotenv`; already-exported variables win.

| Variable | Role |
| --- | --- |
| `MYSQL_HOST_V4` / `MYSQL_HOST_V7` | Twin endpoints |
| `MYSQL_PORT` | Default `3306` |
| `MYSQL_DATABASE` | Default `payments_exp` |
| `MYSQL_USER` | Default `exp_app` |
| `MYSQL_SECRET_ARN` | Primary password source (Secrets Manager JSON) |
| `MYSQL_PASSWORD` | Local override; skips AWS |
| `AWS_REGION` | Default `us-east-1` |
| `EXPERIMENT_ARM` | `v4` or `v7` |
| `TARGET_QPS` | Default `500` |
| `POOL_SIZE` | Default `32` |
| `WARMUP_MINUTES` / `MEASURE_MINUTES` | Fractional minutes allowed |
| `SETTLE_SECONDS` | Default `120` |
| `PRELOAD_BULK_ROWS` / `PRELOAD_SEASON_ROWS` | Hybrid preload sizes |
| `CONFIG_CHECK_PING=0` | Skip DB ping in `config-check` |

## Makefile

| Target | Command |
| --- | --- |
| `config-check` | `go run ./cmd/configcheck` |
| `schema` | `go run ./cmd/schema` |
| `preload-bulk` | `go run ./cmd/preload` |
| `preload-season` | `go run ./cmd/season` |
| `preload` | bulk + season |
| `warmup` | `go run ./cmd/loadgen -mode=warmup` |
| `measure` | `go run ./cmd/loadgen -mode=measure` |
| `export-metrics` | `go run ./cmd/exportmetrics` |
| `run-arm` | warmup + measure + export |
| `tablespace-report` | `go run ./cmd/tablespace` |

`BOTH=1` applies schema / preload / season / tablespace to both hosts.

## Layout

```
cmd/loadgen          warmup + measure
cmd/preload          bulk load
cmd/season           seasoning
cmd/schema           apply SQL
cmd/tablespace       size report
cmd/configcheck      env / ping
cmd/exportmetrics    results table
internal/config      env + Secrets Manager
internal/db          DSN, schema, tablespace
internal/uuidx       v4 / v7 → [16]byte
internal/datagen     synthetic rows + shared preload keys
internal/histogram   HDR microseconds
internal/metrics     results/ JSON
internal/bulk        batch / LOAD DATA
internal/runner      rate-limited single-row INSERT
sql/001_schema.sql
scripts/local-smoke.sh
results/             gitignored JSON
```

## Password resolution

1. If `MYSQL_PASSWORD` is set, use it (local / CI / no AWS).
2. Else fetch `MYSQL_SECRET_ARN` from Secrets Manager. JSON secrets must contain `password` (RDS format). A raw string secret is also accepted.
3. `MYSQL_USER` always comes from env, not from the secret.

No credentials are generated or committed.
