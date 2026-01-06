# disable default rules
.SUFFIXES:
MAKEFLAGS+=-r -R
DATE  = $(shell date +%Y%m%d%H%M%S)
export GOBIN = $(CURDIR)/bin

$(GOBIN)/staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: staticcheck
staticcheck: $(GOBIN)/staticcheck
	$(GOBIN)/staticcheck ./...

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
