# uuid-insert-experiment — rerun from the repo root.
# Go commands load .env themselves (process env wins).

GO      ?= go
PKGS     = ./cmd/... ./internal/...
LDFLAGS ?=

.PHONY: help build test config-check schema \
	preload-bulk preload-season preload \
	warmup measure export-metrics run-arm tablespace-report

help:
	@echo "Rerun:  config-check → schema → preload → warmup → measure → export-metrics"
	@echo ""
	@echo "  make config-check        validate env / optional DB ping"
	@echo "  make schema              apply sql/001_schema.sql (current arm)"
	@echo "  make schema BOTH=1       apply schema to both twins"
	@echo "  make preload-bulk        ~180–190M deterministic rows (or PRELOAD_BULK_ROWS)"
	@echo "  make preload-season      ~10–20M shared seasoning rows"
	@echo "  make preload             preload-bulk + preload-season"
	@echo "  make warmup              inserts + PK lookups (if readers on); discard histograms; settle"
	@echo "  make measure             inserts + PK lookups; HDR insert/read → results/"
	@echo "  readers: READER_POOL_SIZE (default 8; 0 disables), READER_QPS (default 500), READERS=0"
	@echo "  make export-metrics      print results/*.json table"
	@echo "  make run-arm             warmup + measure + export-metrics"
	@echo "  make tablespace-report   information_schema size snapshot"
	@echo "  make test                unit tests (no MySQL required)"
	@echo "  make build               compile all commands into bin/"

build:
	@mkdir -p bin
	$(GO) build $(LDFLAGS) -o bin/loadgen ./cmd/loadgen
	$(GO) build $(LDFLAGS) -o bin/preload ./cmd/preload
	$(GO) build $(LDFLAGS) -o bin/season ./cmd/season
	$(GO) build $(LDFLAGS) -o bin/schema ./cmd/schema
	$(GO) build $(LDFLAGS) -o bin/tablespace ./cmd/tablespace
	$(GO) build $(LDFLAGS) -o bin/configcheck ./cmd/configcheck
	$(GO) build $(LDFLAGS) -o bin/exportmetrics ./cmd/exportmetrics

test:
	$(GO) test $(PKGS)

config-check:
	$(GO) run ./cmd/configcheck

BOTH_FLAG := $(if $(BOTH),-both,)

schema:
	$(GO) run ./cmd/schema $(BOTH_FLAG)

preload-bulk:
	$(GO) run ./cmd/preload $(BOTH_FLAG)

preload-season:
	$(GO) run ./cmd/season $(BOTH_FLAG)

preload: preload-bulk preload-season

warmup:
	$(GO) run ./cmd/loadgen -mode=warmup

measure:
	$(GO) run ./cmd/loadgen -mode=measure

export-metrics:
	$(GO) run ./cmd/exportmetrics

run-arm: warmup measure export-metrics

tablespace-report:
	$(GO) run ./cmd/tablespace $(BOTH_FLAG)
