.SUFFIXES:
MAKEFLAGS+=-r -R

BIN := $(CURDIR)/bin
STATICCHECK_VERSION := v0.7.0
GOLANGCI_LINT_VERSION := v2.12.2
BENCHSTAT_VERSION := v0.0.0-20260112171951-5abaabe9f1bd
export GOBIN := $(BIN)

.PHONY: test
test:
	go test ./...

.PHONY: race
race:
	go test -race ./...

.PHONY: wasm-test
wasm-test:
	GOOS=js GOARCH=wasm go test -exec="$$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./cmd/wasmxsd

.PHONY: fuzz-smoke
fuzz-smoke:
	go test -run '^$$' -fuzz=FuzzXMLStreamParser -fuzztime=10s ./internal/stream
	go test -run '^$$' -fuzz=FuzzSchemaParserLimits -fuzztime=10s ./internal/compile
	go test -run '^$$' -fuzz=FuzzValidateNeverPanics -fuzztime=10s ./internal/validate
	go test -run '^$$' -fuzz=FuzzXSDRegexSyntax -fuzztime=10s ./internal/compile

.PHONY: bench
bench:
	go test -run '^$$' -bench=. -benchmem ./...

.PHONY: bench-smoke
bench-smoke:
	go test -run '^$$' -bench='Benchmark(ParseXSDTime|ValidateIdentityConstraintsRows|ValidateIdentityConstraintsFields|CompileAttributeGroupFanout|CompileSmallSchema)$$' -benchtime=100ms -benchmem ./...

.PHONY: benchstat
benchstat: $(BIN)/benchstat

$(BIN)/benchstat: go.mod Makefile | $(BIN)
	go install golang.org/x/perf/cmd/benchstat@$(BENCHSTAT_VERSION)

.PHONY: xmllint
xmllint: | $(BIN)
	go build -o $(BIN)/xmllint ./cmd/xmllint

.PHONY: wasm
wasm: | docs
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o docs/xsd.wasm ./cmd/wasmxsd
	cp $$(go env GOROOT)/lib/wasm/wasm_exec.js docs/wasm_exec.js

.PHONY: web
web:
	go run ./cmd/xsdweb

.PHONY: web-test
web-test:
	node --test docs/js/validation-flow.test.js

.PHONY: staticcheck
staticcheck: $(BIN)/staticcheck
	$(BIN)/staticcheck ./...

$(BIN)/staticcheck: go.mod Makefile | $(BIN)
	go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)

.PHONY: lint
lint: $(BIN)/golangci-lint
	$(BIN)/golangci-lint run

$(BIN)/golangci-lint: go.mod .golangci.yml Makefile | $(BIN)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(BIN):
	mkdir -p $@

docs:
	mkdir -p $@
