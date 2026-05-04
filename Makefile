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

.PHONY: staticcheck
staticcheck: $(BIN)/staticcheck
	$(BIN)/staticcheck ./...

$(BIN)/staticcheck: go.mod | $(BIN)
	go install honnef.co/go/tools/cmd/staticcheck@latest

$(BIN):
	mkdir -p $@
