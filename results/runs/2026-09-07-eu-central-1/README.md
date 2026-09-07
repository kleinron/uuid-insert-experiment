# Run: 2026-09-07 (eu-central-1)

Ephemeral twin RDS MySQL 8.0.46 experiment: UUIDv4 vs UUIDv7 as `BINARY(16)` PK on `payments`, ~200M preloaded rows (shared `KeyAt`), measure 20 min @ 500 inserts/s + 500 PK lookups/s.

## Headline

| | v4 | v7 |
|---|---:|---:|
| Insert p50 / p95 | 2031 / 2503 µs | 1552 / 1888 µs (~−24% / −25%) |
| Read p50 / p95 | 695 / 1391 µs | 754 / 1407 µs (~flat) |
| CW Read IOPS (avg) | ~474 | ~238 (−50%) |
| CW Write IOPS (avg) | ~948 | ~511 (−46%) |
| CPU % (avg) | ~15.5 | ~15.9 |
| `DATA_LENGTH` | 27.677 GiB | 27.656 GiB |

## Files

| File | Description |
|------|-------------|
| `measure-v4.json` / `measure-v7.json` | Loadgen HDR results (insert + read) |
| `measure-cloudwatch.json` | CloudWatch avgs for exact measure windows |
| `twin-deep-analysis.md` | Full twin analysis write-up |
| `twin-metrics.json` | Structured live InnoDB / tablespace dump |
| `innodb-status-v4.txt` / `innodb-status-v7.txt` | Truncated `SHOW ENGINE INNODB STATUS` |

Infra: `db.r6g.large`, 400 GiB gp3 @ 12k IOPS, BP 12 GiB, durable flush, AHI off, doublewrite off. Sequential v4 then v7.
