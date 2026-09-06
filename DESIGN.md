# Design

This repo is the **app / preload / measure harness only**. Twin RDS, networking, and the loadgen instance are separate infra. **No Terraform lives here.**

Locked protocol: **hybrid@120** — hybrid preload (bulk, then identical season) and a **120s** client settle after warmup (`SETTLE_SECONDS=120`).

## Phases

```
                    ┌─ MYSQL_HOST_V4 ─┐
 schema ──► preload ┤                 ├──► tablespace-report
                    └─ MYSQL_HOST_V7 ─┘            │
                                                   ▼
                              if measured data+index ≥ 50 GiB → revisit instance/BP
                                                   │
                    EXPERIMENT_ARM = v4 | v7       ▼
                    warmup (uuidx inserts + PK lookups, discard HDR) ──► settle 120s
                                                   │
                                                   ▼
                              measure (uuidx inserts + PK lookups, HDR insert/read → results/) ──► export
```

1. **schema** — `sql/001_schema.sql` on both twins (`BOTH=1`).
2. **preload-bulk** — ~180–190M rows, shared `datagen.KeyAt` (not v4/v7).
3. **preload-season** — ~10–20M more rows, same key source, same `n` sequence on both twins.
4. **tablespace-report** — after preload, before measure. If measured `DATA_LENGTH + INDEX_LENGTH` ≥ **50 GiB**, stop and **revisit instance class / InnoDB buffer pool (BP)** with the AWS SA.
5. **warmup** — full `TARGET_QPS` inserts with arm-typed UUIDs, plus concurrent PK point lookups at `READER_QPS` when readers are enabled; histograms discarded.
6. **settle** — `SETTLE_SECONDS=120`.
7. **measure** — 20 minutes of the same mixed insert+lookup load; separate HDR p50/p95/p99/p999 for `op=insert` and `op=read` → `results/`.

Warmup and measure keep inserts and reads in the same window (not serialized phases). Readers `SELECT` by `payment_id` only, sampling uniformly from the deterministic preload key space so lookups hit already-loaded rows rather than just-inserted measure keys. `READER_POOL_SIZE=0` or `READERS=0` is write-only. Cheaper sequential-ish v7 inserts are expected to leave more buffer-pool / I/O headroom than random v4 inserts, which should appear as lower concurrent read tail latency once the table is much larger than memory.

Rerun order matches the Makefile: config-check → schema → preload → warmup → measure → export-metrics.

## Authoritative PK rules

| Phase | Generator | Twin identity |
| --- | --- | --- |
| Preload bulk + season | `datagen.KeyAt(n)` only. Deterministic 16-byte SHA-256 of a fixed domain + row index. **Not** UUIDv4 or UUIDv7. | **Identical** on both twins for the same `n`. Same `PRELOAD_BULK_ROWS` / `PRELOAD_SEASON_ROWS`. |
| Warmup + measure **inserts** | `internal/uuidx` UUIDv4 or UUIDv7 → `[16]byte`, selected by `EXPERIMENT_ARM`. | **Arm-only.** v4 host gets v4 keys; v7 host gets v7 keys. |
| Warmup + measure **reads** | `datagen.KeyAt(n)` for uniform random `n` in `[0, PRELOAD_BULK_ROWS + PRELOAD_SEASON_ROWS)`. Point lookup on `payment_id` only. | **Shared** with preload. Not just-inserted measure UUIDs. |

Do not mix arm-typed UUIDs into preload. Do not use `KeyAt` as the measure **insert** key. Concurrent readers may (and should) look up `KeyAt` rows so the read mix is independent of insert-page locality.

Bulk `n = 0 .. PRELOAD_BULK_ROWS-1`. Season continues at `n = PRELOAD_BULK_ROWS`.

## Infra boundary

- Hosts, secret ARN, TLS, region: env (see `scripts/export_env_from_tf.sh` for the locked TF output names).
- Password: `MYSQL_SECRET_ARN` (primary) or `MYSQL_PASSWORD` (local). Never a plaintext TF `mysql_password`.
- `MYSQL_TLS` defaults **false** (same-VPC). When true, the client uses `tls=skip-verify`.
- `LOADGEN_INSTANCE_HINT` / `LOADGEN_AZ` are optional ops notes (unused by Go). `LOADGEN_AZ` must match the RDS AZ.
- No `rds_endpoint_*` aliases. No Terraform in this repository.
