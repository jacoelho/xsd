# disable default rules
.SUFFIXES:
MAKEFLAGS+=-r -R
export GOBIN = $(CURDIR)/bin
GML_INSTANCE_PATH = testdata/gml/example.gml
GML_ENTRY_SCHEMA = testdata/gml/xsd/LandCoverVector.xsd
GML_INSTANCE_MAX_TOKEN_SIZE = 134217728
GML_CACHE_PATH ?=
GML_FORCE_DOWNLOAD ?= false
XMLLINT_SOURCE_DIRS = . cmd internal
XMLLINT_SOURCES = $(shell find cmd internal -type f -name '*.go') $(shell find . -maxdepth 1 -type f -name '*.go')

$(GOBIN)/staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: xmllint
xmllint: $(GOBIN)/xmllint

$(GOBIN)/xmllint: $(XMLLINT_SOURCE_DIRS) $(XMLLINT_SOURCES) go.mod
	go build -o $@ ./cmd/xmllint

.PHONY: staticcheck
staticcheck: $(GOBIN)/staticcheck
	$(GOBIN)/staticcheck ./...

testdata/xsdtests:
	git clone --depth 1 https://github.com/w3c/xsdtests.git testdata/xsdtests

.PHONY: w3c
w3c: testdata/xsdtests
	go test -timeout 2m -tags w3c -run ^TestW3CConformance github.com/jacoelho/xsd/w3c --count=1

.PHONY: test
test: testdata/xsdtests
	go test -timeout 2m -race -shuffle=on ./...

.PHONY: gml
gml: $(GOBIN)/xmllint
	@cacheArgs=""; \
	if [ -n "$(GML_CACHE_PATH)" ]; then \
		cacheArgs="-gml-cache '$(GML_CACHE_PATH)'"; \
	fi; \
	if [ "$(GML_FORCE_DOWNLOAD)" = "true" ] || [ "$(GML_FORCE_DOWNLOAD)" = "1" ]; then \
		go run ./testdata/gml/setup.go prepare -force-download $$cacheArgs; \
	else \
		go run ./testdata/gml/setup.go prepare -skip-download $$cacheArgs; \
	fi
	/usr/bin/time $(GOBIN)/xmllint --schema "$(GML_ENTRY_SCHEMA)" --instance-max-token-size "$(GML_INSTANCE_MAX_TOKEN_SIZE)" "$(GML_INSTANCE_PATH)"

.PHONY: gml-download
gml-download: $(GOBIN)/xmllint
	go run ./testdata/gml/setup.go prepare -force-download
	/usr/bin/time $(GOBIN)/xmllint --schema "$(GML_ENTRY_SCHEMA)" --instance-max-token-size "$(GML_INSTANCE_MAX_TOKEN_SIZE)" "$(GML_INSTANCE_PATH)"
