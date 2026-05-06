.SUFFIXES:
MAKEFLAGS+=-r -R

BIN := $(CURDIR)/bin
XSD_GO := $(wildcard *.go)
XMLLINT_GO := cmd/xmllint/main.go
export GOBIN := $(BIN)

.PHONY: test
test:
	go test ./...

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

$(BIN)/staticcheck: go.mod | $(BIN)
	go install honnef.co/go/tools/cmd/staticcheck@latest

$(BIN):
	mkdir -p $@

docs:
	mkdir -p $@
