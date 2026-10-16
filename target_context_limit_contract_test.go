package xsd_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestReferenceFreeSchemasRespectTargetContextLimit(t *testing.T) {
	t.Parallel()
	sources := []xsd.SchemaSource{
		xsd.Bytes("a.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a"><xs:element name="a" type="xs:string"/></xs:schema>`)),
		xsd.Bytes("b.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:b"><xs:element name="b" type="xs:string"/></xs:schema>`)),
	}
	for _, limit := range []int{1, 2} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			t.Parallel()
			engine, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTargetContexts: limit}, sources...)
			if limit == 1 {
				xerr, ok := errors.AsType[*xsderrors.Error](err)
				if engine != nil || !ok || xerr.Category() != xsderrors.CategorySchemaCompile || xerr.Code() != xsderrors.CodeSchemaLimit || xerr.Message() != "schema target contexts exceed MaxSchemaTargetContexts" {
					t.Fatalf("CompileWithOptions(limit 1) = %v, %v, want nil engine and schema target-context limit", engine, err)
				}
				return
			}
			if err != nil || engine == nil {
				t.Fatalf("CompileWithOptions(limit 2) = %v, %v, want engine", engine, err)
			}
			for _, document := range []string{`<a xmlns="urn:a">v</a>`, `<b xmlns="urn:b">v</b>`} {
				if err := engine.Validate(strings.NewReader(document)); err != nil {
					t.Fatalf("Validate(%q) = %v", document, err)
				}
			}
		})
	}
}
