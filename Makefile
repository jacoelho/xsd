# disable default rules
.SUFFIXES:
MAKEFLAGS+=-r -R
DATE  = $(shell date +%Y%m%d%H%M%S)
export GOBIN = $(CURDIR)/bin

$(GOBIN)/staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: xmllint
xmllint: $(GOBIN)/xmllint

$(GOBIN)/xmllint:
	go build -o $(GOBIN)/xmllint ./cmd/xmllint

.PHONY: staticcheck
staticcheck: $(GOBIN)/staticcheck
	$(GOBIN)/staticcheck ./...

.PHONY: fieldalignment
fieldalignment:
	go run golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest ./...

testdata/xsdtests:
	git clone --depth 1 https://github.com/w3c/xsdtests.git testdata/xsdtests

.PHONY: testdata
testdata: testdata/xsdtests

.PHONY: w3c
w3c: testdata/xsdtests
	go test -timeout 2m -run ^TestW3CConformance github.com/jacoelho/xsd/w3c --count=1

.PHONY: test
test: testdata/xsdtests
	go test -timeout 2m -race -shuffle=on ./...
