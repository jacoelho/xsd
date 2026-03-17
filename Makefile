# disable default rules
.SUFFIXES:
MAKEFLAGS+=-r -R
export GOBIN = $(CURDIR)/bin
GML_INSTANCE_PATH = testdata/gml/example.gml
GML_ENTRY_SCHEMA = testdata/gml/xsd/LandCoverVector.xsd
GML_INSTANCE_MAX_TOKEN_SIZE = 134217728

$(GOBIN)/staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest

.PHONY: xmllint
xmllint:
	go build -o $(GOBIN)/xmllint ./cmd/xmllint

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
gml: xmllint
	go run ./testdata/gml/setup.go prepare
	/usr/bin/time $(GOBIN)/xmllint --schema "$(GML_ENTRY_SCHEMA)" --instance-max-token-size "$(GML_INSTANCE_MAX_TOKEN_SIZE)" "$(GML_INSTANCE_PATH)"
