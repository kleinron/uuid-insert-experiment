#!/usr/bin/env bash
# Small-scale path: 1k bulk + short warmup/measure. No AWS.
# Requires a reachable MySQL and MYSQL_PASSWORD.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

if [[ -f .env ]]; then
  echo "loading .env (existing process env wins inside the Go commands)"
fi

export MYSQL_HOST_V4="${MYSQL_HOST_V4:-127.0.0.1}"
export MYSQL_HOST_V7="${MYSQL_HOST_V7:-${MYSQL_HOST_V4}}"
export MYSQL_PORT="${MYSQL_PORT:-3306}"
export MYSQL_DATABASE="${MYSQL_DATABASE:-payments_exp}"
export MYSQL_USER="${MYSQL_USER:-exp_app}"
export EXPERIMENT_ARM="${EXPERIMENT_ARM:-v4}"
export TARGET_QPS="${TARGET_QPS:-50}"
export POOL_SIZE="${POOL_SIZE:-4}"
export WARMUP_MINUTES="${WARMUP_MINUTES:-0.05}"
export MEASURE_MINUTES="${MEASURE_MINUTES:-0.1}"
export SETTLE_SECONDS="${SETTLE_SECONDS:-0}"
export PRELOAD_BULK_ROWS="${PRELOAD_BULK_ROWS:-1000}"
export PRELOAD_SEASON_ROWS="${PRELOAD_SEASON_ROWS:-100}"
export PRELOAD_BATCH_SIZE="${PRELOAD_BATCH_SIZE:-100}"
export PRELOAD_WORKERS="${PRELOAD_WORKERS:-2}"
export READER_POOL_SIZE="${READER_POOL_SIZE:-2}"
export READER_QPS="${READER_QPS:-50}"

if [[ -z "${MYSQL_PASSWORD:-}" ]]; then
  echo "Set MYSQL_PASSWORD for the local smoke run (Secrets Manager is not used)." >&2
  echo "Example MySQL (Docker):" >&2
  echo "  docker run --name payments-exp-mysql -e MYSQL_ROOT_PASSWORD=dev \\" >&2
  echo "    -e MYSQL_DATABASE=payments_exp -e MYSQL_USER=exp_app -e MYSQL_PASSWORD=dev \\" >&2
  echo "    -p 3306:3306 -d mysql:8.0 --local-infile=1" >&2
  exit 1
fi

make config-check
make schema
make preload
make warmup
make measure
make export-metrics
make tablespace-report
echo "local smoke finished; see results/"
