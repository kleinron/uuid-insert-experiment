# UUID twin deep analysis — payments-exp v4 vs v7

**Generated:** 2026-09-07 ~12:17 Asia/Jerusalem (09:16 UTC collection)  
**Artifacts:** `/workspace/uuid-twin-metrics.json`, `/workspace/v4-latest.json`, `/workspace/v7-latest.json`, `/workspace/innodb-status-v4.txt`, `/workspace/innodb-status-v7.txt`  
**Source:** live loadgen `payments-exp-loadgen` (`i-0f792d816c3f2c815` / `3.65.1.188`) + RDS describe + harness `tablespace` / custom `deepmetrics` collector.

---

## Executive summary

Side-by-side measure on twin MySQL 8.0.46 RDS instances (`db.r6g.large`, 400 GiB gp3 @ 12k IOPS, shared parameter group `payments-exp-mysql80`) shows **UUIDv7 inserts are materially faster than UUIDv4** under the locked hybrid@120 protocol, while **PK point-lookup reads are essentially similar** (v4 slightly better at p50).

| Metric | v4 | v7 | Δ (v7 vs v4) |
| --- | ---: | ---: | ---: |
| Insert p50 | 2031 µs | 1552 µs | **−23.6%** |
| Insert p95 | 2503 µs | 1888 µs | **−24.6%** |
| Insert p99 | 4319 µs | 3851 µs | **−10.8%** |
| Insert p999 | 28175 µs | 19711 µs | **−30.0%** |
| Read p50 | 695 µs | 754 µs | +8.5% |
| Read p95 | 1391 µs | 1407 µs | +1.2% |
| Read p99 | 1920 µs | 1971 µs | +2.7% |
| Tablespace `DATA_LENGTH` | 27.677 GiB | 27.656 GiB | −21 MiB (−0.07%) |
| `AVG_ROW_LENGTH` (estimate) | 150 | 139 | −11 |
| BP hit ratio (cumulative) | 97.085% | 97.145% | +0.06 pp |

InnoDB **runtime variables match** on both arms. Cumulative status counters since ~16.7 h uptime are nearly twin-symmetric (same preload volume). Physical clustered index size is almost identical because **preload dominates** (~200 M shared `KeyAt` rows); only warmup+measure inserts use arm-typed UUIDs. The insert latency gap is therefore explained by **insert-time page locality / split behavior**, not by a large final size delta.

---

## Experiment context (what was measured)

Protocol (from harness `DESIGN.md` / `configcheck`):

- **Twins:** `payments-exp-v4` / `payments-exp-v7` (eu-central-1a)
- **Preload:** identical deterministic `KeyAt` bulk (~185 M) + season (~15 M) on both — **not** UUID v4/v7
- **Warmup:** 25 min @ 500 insert QPS + 500 read QPS (histograms discarded)
- **Settle:** 120 s
- **Measure:** 20 min @ target 500 insert QPS (pool 32) + 500 PK lookup QPS (pool 8)
- **Insert keys (warmup/measure):** arm UUID (`uuidx` v4 vs v7) as `BINARY(16)` PK
- **Read keys:** uniform sample of preload `KeyAt` space (independent of just-inserted UUIDs)
- **Run order:** v4 measure `2026-09-07T06:52–07:12Z`, then v7 `07:39–07:59Z` (sequential, not simultaneous)

Schema (identical):

```sql
CREATE TABLE `payments` (
  `payment_id` binary(16) NOT NULL,
  `merchant_id` varchar(30) NOT NULL,
  `customer_id` varchar(30) NOT NULL,
  `amount` decimal(12,2) NOT NULL,
  `currency` char(3) NOT NULL,
  `reference_id` varchar(30) DEFAULT NULL,
  PRIMARY KEY (`payment_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
```

---

## Latency results (from results JSON)

### Inserts

| Arm | Actual QPS | N | Errors | p50 µs | p95 µs | p99 µs | p999 µs | max µs |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| **v4** | 499.96 | 599954 | 0 | 2031 | 2503 | 4319 | 28175 | 118207 |
| **v7** | 499.97 | 599965 | 0 | 1552 | 1888 | 3851 | 19711 | 120703 |

### Reads (PK point lookup)

| Arm | Actual QPS | N | Errors | p50 µs | p95 µs | p99 µs | p999 µs | max µs |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| **v4** | 499.54 | 599452 | 0 | 695 | 1391 | 1920 | 8943 | 111807 |
| **v7** | 499.69 | 599625 | 0 | 754 | 1407 | 1971 | 8415 | 80511 |

Both arms sustained target QPS with **zero errors**.

---

## Storage / tablespace comparison

From `EXPERIMENT_ARM=… ./bin/tablespace`, `information_schema.TABLES`, `SHOW TABLE STATUS`, and `mysql.innodb_table_stats` / `innodb_index_stats` (all live at collection):

| Field | v4 | v7 |
| --- | ---: | ---: |
| `TABLE_ROWS` (estimate) | 197 316 213 | 212 980 348 |
| `innodb_table_stats.n_rows` | 197 316 213 | 212 980 348 |
| `DATA_LENGTH` | 29 717 528 576 (27.677 GiB) | 29 695 459 328 (27.656 GiB) |
| `INDEX_LENGTH` | 0 (clustered PK counted in data) | 0 |
| `DATA_FREE` | 4 194 304 | 5 242 880 |
| `AVG_ROW_LENGTH` | 150 | 139 |
| `clustered_index_size` (pages) | 1 813 814 | 1 812 467 |
| `n_leaf_pages` | 1 582 962 | 1 581 733 |
| bytes / index page | **16384.0** (exact 16 KiB) | **16384.0** |
| est. rows / leaf page | ~124.65 | ~134.65 |
| Engine / row_format | InnoDB / Dynamic | InnoDB / Dynamic |

**Notes on row counts:** Exact `SELECT COUNT(*)` was **not** run (too slow at ~200 M rows). Estimates disagree by ~15 M rows despite nearly identical physical size and nearly identical cumulative `Innodb_rows_inserted` (~203.727 M on both since uptime). Treat `TABLE_ROWS` / `n_rows` as **approximate**; prefer `DATA_LENGTH` + leaf/page counts for packing discussion.

`DATA_FREE` is tiny on both (~4–5 MiB) — no large free-extent story. The interesting signal is the **estimate packing** (more estimated rows/leaf on v7) while **physical GiB stays within 0.07%**.

---

## InnoDB config confirmation (should match)

| Variable | v4 | v7 | Match? |
| --- | ---: | ---: | --- |
| `VERSION()` | 8.0.46 | 8.0.46 | yes |
| `innodb_buffer_pool_size` | 12 884 901 888 (12 GiB) | same | yes |
| `innodb_io_capacity` | 5000 | 5000 | yes |
| `innodb_io_capacity_max` | 12000 | 12000 | yes |
| `innodb_flush_log_at_trx_commit` | 1 | 1 | yes |
| `sync_binlog` | 1 | 1 | yes |
| `innodb_redo_log_capacity` | 2 147 483 648 (2 GiB) | same | yes |
| `innodb_adaptive_hash_index` | OFF (`0`) | OFF | yes |
| `innodb_flush_neighbors` | 0 | 0 | yes |
| `innodb_doublewrite` | OFF | OFF | yes |

### RDS describe-db-instances

| Attr | v4 | v7 |
| --- | --- | --- |
| Class | db.r6g.large | db.r6g.large |
| Engine | mysql 8.0.46 | mysql 8.0.46 |
| Storage | 400 GiB gp3, 12000 IOPS, 500 MB/s | same |
| Param group | `payments-exp-mysql80` (in-sync) | same |
| Multi-AZ | false | false |
| AZ | eu-central-1a | eu-central-1a |
| Perf Insights | enabled | enabled |
| Enhanced monitoring interval | 60 s | 60 s |
| Backup retention | 0 | 0 |

CloudWatch `FreeStorageSpace` not pulled (may be AccessDenied); skipped per brief. PI/EM deep dig skipped (optional).

---

## Runtime status counters + hit ratios

Collected ~09:16 UTC via `SHOW GLOBAL STATUS` (curated). **Uptime ≈ 60139 s (v4) / 60161 s (v7)** — counters are **lifetime**, not measure-window-only.

| Counter | v4 | v7 |
| --- | ---: | ---: |
| `Innodb_buffer_pool_reads` | 30 534 234 | 29 883 153 |
| `Innodb_buffer_pool_read_requests` | 1 047 560 815 | 1 046 873 070 |
| **BP hit ratio** `1 − reads/requests` | **0.970852** | **0.971455** |
| `Innodb_buffer_pool_pages_data` | 778166 | 778166 |
| `Innodb_buffer_pool_pages_free` | 8192 | 8192 |
| `Innodb_buffer_pool_pages_dirty` | **77698** | **376** |
| `Innodb_buffer_pool_pages_total` | 786432 | 786432 |
| `Innodb_buffer_pool_wait_free` | 265 | 387 |
| `Innodb_data_reads` | 30 535 511 | 29 884 766 |
| `Innodb_data_writes` | 76 406 392 | 75 155 783 |
| `Innodb_data_read` (bytes) | 500 266 224 128 | 489 609 677 312 |
| `Innodb_data_written` (bytes) | 1 000 552 686 592 | 980 257 527 808 |
| `Innodb_pages_written` | 57 308 574 | 56 074 212 |
| `Innodb_pages_read` | 30 535 998 | 29 885 239 |
| `Innodb_log_writes` | 19 073 339 | 19 060 710 |
| `Innodb_os_log_written` | 49 940 799 488 | 49 829 357 568 |
| `Innodb_os_log_fsyncs` | 5 752 068 | 5 669 813 |
| `Innodb_dblwr_writes` | 0 | 0 |
| `Innodb_row_lock_waits` / `time` | 0 / 0 | 0 / 0 |
| `Innodb_rows_inserted` | 203 727 095 | 203 727 503 |
| `Innodb_rows_read` | 3 732 500 | 3 733 139 |
| `Handler_read_key` | 1 453 785 | 1 453 969 |
| `Handler_read_rnd` | 0 | 0 |
| `Com_insert` | 1 550 133 | 1 550 153 |
| `Com_select` | 1 504 707 | 1 504 899 |
| `Questions` | 3 060 044 | 3 060 268 |
| `Slow_queries` | 17 | 12 |
| `Threads_running` | 2 | 2 |

`SHOW ENGINE INNODB STATUS` (truncated sections saved under `/workspace/innodb-status-{v4,v7}.txt`):

- Aggregate BP hit rate snapshot: **v4 983/1000**, **v7 994/1000**
- Pages created: v4 1 600 393 vs v7 1 595 988
- Modified (dirty) pages at snapshot: **v4 77 698 vs v7 376** (v4 still much dirtier long after measure — worth noting; may reflect random-page dirtying / checkpoint state, not measure QPS)

---

## Interpretation

### Why insert latency differed

1. **PK insert locality:** UUIDv4 scatters inserts across the clustered index leaf space → more random page reads, more leaf splits, worse BP reuse for the write path. UUIDv7 is time-ordered → inserts concentrate near the right edge → fewer random reads and cheaper page maintenance at 500 QPS.
2. **Observed latency shape:** ~24% better p50/p95 and ~30% better p999 on v7, with both arms still meeting target QPS — classic “same durable commit settings, different index maintenance cost” pattern (`innodb_flush_log_at_trx_commit=1`, `sync_binlog=1` identical).
3. **Not a config skew:** variables, instance class, storage, and param group match.
4. **Not a large tablespace win yet:** preload is shared and dominates size; measure inserts are only ~600 k rows (~0.3% of table). Final `DATA_LENGTH` barely differs. The win is **during** random vs sequential-ish insert I/O, not final GiB.

### Why reads were similar

Readers look up **preload `KeyAt` keys**, not the just-inserted UUID stream. That workload is identical on both twins and largely independent of insert-page locality. Cumulative BP hit ratios are already ~97% on both; read p95/p99 within ~1–3%. Slight v4 p50 edge (+8.5% worse on v7) is small vs the insert gap and could be noise / sequential-run effects (v7 measured later, different dirty-page / cache state).

### What storage says about fragmentation / page fill

- Physical size ≈ **27.67 GiB both**; page accounting exact at 16 KiB/page.
- InnoDB **estimates** imply denser packing on v7 (~135 vs ~125 rows/leaf) and lower `AVG_ROW_LENGTH` (139 vs 150), but row estimates are inconsistent with nearly equal `Innodb_rows_inserted` and equal preload — **do not over-read the row estimate delta**.
- `DATA_FREE` is negligible on both → no evidence of large free space holes; any v4 fragmentation from random measure inserts is a small fraction of the tree.
- Expect a clearer size/fill divergence only if measure/warmup UUID volume becomes a much larger share of the table, or if a pure-UUID load (no shared KeyAt preload) is used.

---

## Caveats

1. **`SHOW GLOBAL STATUS` is cumulative since Uptime (~16.7 h)** — includes preload, warmup, settle, measure, and idle. Do not treat counter deltas as measure-only without a before/after snapshot.
2. **`TABLE_ROWS` / `innodb_*_stats` are estimates** and currently disagree across arms despite similar physical size; exact `COUNT(*)` not taken.
3. **Sequential run order:** v4 then v7 — time-of-day / cache / dirty-page state not simultaneous. Dirty-page asymmetry at collection (77 k vs 376) underscores post-run state drift.
4. **Doublewrite OFF** on both (experiment param group) — fine for twin fairness, not a production durability recommendation.
5. **AHI OFF** on both — removes one adaptive-cache confounder.
6. PI/EM and FreeStorageSpace deep pulls skipped / not required for this dump.

---

## Recommendations

1. **Follow-up A — measure-window counters:** snapshot curated `SHOW GLOBAL STATUS` immediately before/after each arm’s measure to attribute BP reads, pages written, and log bytes to the 20 min window only.
2. **Follow-up B — pure UUID preload:** optional experiment without shared `KeyAt` preload (or much longer UUID insert phase) to amplify tablespace / fill-factor divergence.
3. **Follow-up C — concurrent twin measure:** run both arms at once from two loadgens (or carefully partitioned pools) to remove sequential-order confounds.
4. **Follow-up D — read mix:** add a reader that looks up **recently inserted** UUIDs to test whether v7’s right-edge locality also helps “read your writes.”
5. **Teardown reminder:** when analysis is done, tear down twin RDS + loadgen to stop gp3/r6g spend (`payments-exp-*` in eu-central-1); backup retention is already 0.
6. Keep results JSON + this report as the locked baseline before changing instance class, BP size, or doublewrite.

---

## File index

| Path | Contents |
| --- | --- |
| `/workspace/uuid-twin-deep-analysis.md` | This report |
| `/workspace/uuid-twin-metrics.json` | Structured metrics dump (both arms + deltas + RDS) |
| `/workspace/v4-latest.json` / `v7-latest.json` | Loadgen measure HDR results |
| `/workspace/innodb-status-v4.txt` / `v7.txt` | Truncated ENGINE INNODB STATUS sections |
| `/workspace/rds-v4.json` / `rds-v7.json` | Raw `describe-db-instances` |
| `/workspace/deepmetrics-out.json` | Raw collector output |
