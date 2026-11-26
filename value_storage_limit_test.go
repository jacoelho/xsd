package xsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func minInclusiveChain(depth int) []byte {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	base := "xs:decimal"
	for i := range depth {
		name := fmt.Sprintf("t%d", i)
		fmt.Fprintf(&b, `<xs:simpleType name="%s"><xs:restriction base="%s"><xs:minInclusive value="%d"/></xs:restriction></xs:simpleType>`, name, base, i)
		base = name
	}
	b.WriteString(`</xs:schema>`)
	return []byte(b.String())
}

func TestCompileValueStorageLimitNamesOptions(t *testing.T) {
	t.Parallel()

	reduced := xsd.CompileOptions{MaxSchemaTotalBytes: 128 << 10, MaxSchemaInstantiatedNodes: 4000}
	_, err := xsd.CompileWithOptions(reduced, xsd.Bytes("chain.xsd", minInclusiveChain(600)))
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) || xerr.Code() != xsderrors.CodeSchemaLimit {
		t.Fatalf("600-deep chain under reduced options = %v, want schema.limit", err)
	}
	if !strings.Contains(err.Error(), "MaxSchemaTotalBytes") || !strings.Contains(err.Error(), "MaxSchemaInstantiatedNodes") {
		t.Fatalf("error %q does not name the options sizing the value budget", err)
	}
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{}, xsd.Bytes("chain.xsd", minInclusiveChain(600))); err != nil {
		t.Fatalf("600-deep chain under default options = %v", err)
	}
	if _, err := xsd.CompileWithOptions(reduced, xsd.Bytes("chain.xsd", minInclusiveChain(50))); err != nil {
		t.Fatalf("50-deep chain under reduced options = %v", err)
	}
}
