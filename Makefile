.SUFFIXES:
MAKEFLAGS+=-r -R

BIN := $(CURDIR)/bin
XSD_GO := $(wildcard *.go)
XMLLINT_GO := cmd/xmllint/main.go
STATICCHECK_VERSION := v0.7.0
GOLANGCI_LINT_VERSION := v2.11.4
export GOBIN := $(BIN)

.PHONY: test
test:
	go test ./...

.PHONY: race
race:
	go test -race ./...

.PHONY: fuzz-smoke
fuzz-smoke:
	go test -run '^$$' -fuzz=FuzzXMLStreamParser -fuzztime=10s .
	go test -run '^$$' -fuzz=FuzzSchemaParserLimits -fuzztime=10s .
	go test -run '^$$' -fuzz=FuzzValidateNeverPanics -fuzztime=10s .
	go test -run '^$$' -fuzz=FuzzXSDRegexSyntax -fuzztime=10s .

.PHONY: bench
bench:
	go test -run '^$$' -bench=. -benchmem ./...

.PHONY: bench-smoke
bench-smoke:
	go test -run '^$$' -bench='Benchmark(ParseXSDTime|ValidateIdentityConstraintsRows|ValidateIdentityConstraintsFields|CompileAttributeGroupFanout|CompileSmallSchema)$$' -benchtime=100ms -benchmem .

.PHONY: xmllint
xmllint: $(BIN)/xmllint

$(BIN)/xmllint: $(XSD_GO) $(XMLLINT_GO) go.mod | $(BIN)
	go build -o $@ ./cmd/xmllint

.PHONY: wasm
wasm: | docs
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o docs/xsd.wasm ./cmd/wasmxsd
	cp $$(go env GOROOT)/lib/wasm/wasm_exec.js docs/wasm_exec.js

.PHONY: web
web:
	go run ./cmd/xsdweb

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
