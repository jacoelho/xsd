.SUFFIXES:
MAKEFLAGS+=-r -R

BIN := $(CURDIR)/bin
STATICCHECK_VERSION := v0.8.2
GOLANGCI_LINT_VERSION := v2.15.0
BENCHSTAT_VERSION := v0.0.0-20260112171951-5abaabe9f1bd
export GOBIN := $(BIN)

.PHONY: test
test: vet-32bit
	go test ./...

.PHONY: vet-32bit
vet-32bit:
	GOOS=linux GOARCH=386 go vet ./...

.PHONY: race
race:
	go test -race ./...

.PHONY: wasm-test
wasm-test:
	GOOS=js GOARCH=wasm go test -exec="$$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./cmd/wasmxsd

.PHONY: fuzz-smoke
fuzz-smoke:
	go test -run '^$$' -fuzz='^FuzzReader$$' -fuzztime=10s ./internal/xmlstream
	go test -run '^$$' -fuzz='^FuzzSchemaParserLimits$$' -fuzztime=10s ./internal/schema
	go test -run '^$$' -fuzz='^FuzzValidateNeverPanics$$' -fuzztime=10s ./internal/validate
	go test -run '^$$' -fuzz='^FuzzXSDRegexSyntax$$' -fuzztime=10s ./internal/xsdregex

.PHONY: bench
bench:
	go test -run '^$$' -bench=. -benchmem ./...

.PHONY: bench-smoke
bench-smoke:
	go test -run '^$$' -bench=. -benchtime=2x -benchmem ./...

LIBXML2 ?= /usr/bin/xmllint
LARGE_RUNS ?= 10

.PHONY: bench-libxml2
bench-libxml2: xmllint
	$(LIBXML2) --version
	XSD_LARGE_BENCHMARK=1 XSD_LARGE_RUNS=$(LARGE_RUNS) XSD_LARGE_LIBXML2=$(LIBXML2) \
		go test ./tests -run '^TestLargeXMLLintBenchmark$$' -timeout=90m -count=1 -v

.PHONY: benchstat
benchstat: $(BIN)/benchstat

$(BIN)/benchstat: go.mod Makefile | $(BIN)
	go install golang.org/x/perf/cmd/benchstat@$(BENCHSTAT_VERSION)

.PHONY: xmllint
xmllint: | $(BIN)
	go build -o $(BIN)/xmllint ./cmd/xmllint

.PHONY: wasm
wasm:
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o web/xsd.wasm ./cmd/wasmxsd
	cp $$(go env GOROOT)/lib/wasm/wasm_exec.js web/wasm_exec.js

.PHONY: web
web: wasm
	go run ./cmd/xsdweb

.PHONY: web-test
web-test:
	node --test web/js/*.test.js

.PHONY: browser-test
browser-test:
	npm --prefix web/js run test:browser

.PHONY: oracles
oracles:
	docker build --tag xsd-oracles:local tools/oracles

ORACLE_JOBS ?= 6
ORACLE_CASES ?=

.PHONY: oracle-measure
oracle-measure:
	go run ./tools/oracles/measure -j $(ORACLE_JOBS) $(ORACLE_CASES)

.PHONY: staticcheck
staticcheck: $(BIN)/staticcheck
	$(BIN)/staticcheck ./...

$(BIN)/staticcheck: go.mod Makefile | $(BIN)
	go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)


.PHONY: lint-config
lint-config: $(BIN)/golangci-lint
	$(BIN)/golangci-lint config verify

.PHONY: lint
lint: lint-config
	$(BIN)/golangci-lint run

$(BIN)/golangci-lint: go.mod .golangci.yml Makefile | $(BIN)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(BIN):
	mkdir -p $@
