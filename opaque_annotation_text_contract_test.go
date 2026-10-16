package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestOpaqueAnnotationCumulativeTextLimit(t *testing.T) {
	for _, test := range []struct {
		name        string
		secondBytes int
		wantLimit   bool
	}{
		{name: "at cumulative limit", secondBytes: 64},
		{name: "above cumulative limit", secondBytes: 65, wantLimit: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>` +
				strings.Repeat("a", 64) + `<?p?>` + strings.Repeat("b", test.secondBytes) +
				`</xs:documentation></xs:annotation><xs:element name="r" type="xs:string"/></xs:schema>`
			engine, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTokenBytes: 128},
				xsd.Bytes("opaque-cumulative.xsd", []byte(schema)))
			if test.wantLimit {
				diagnostic, ok := errors.AsType[*xsderrors.Error](err)
				if engine != nil || !ok {
					t.Fatalf("CompileWithOptions() = %v, %v; want nil engine and structured limit error", engine, err)
				}
				if diagnostic.Category() != xsderrors.CategorySchemaParse || diagnostic.Code() != xsderrors.CodeSchemaLimit ||
					diagnostic.Message() != "schema XML text exceeds configured limit" || diagnostic.Path() != "opaque-cumulative.xsd" ||
					diagnostic.Line() != 1 || diagnostic.Column() != 158 || diagnostic.Cause() != nil {
					t.Fatalf("CompileWithOptions() diagnostic = %v; want schema parse/limit at opaque-cumulative.xsd:1:158 without cause", err)
				}
				return
			}
			if err != nil || engine == nil {
				t.Fatalf("CompileWithOptions() = %v, %v; want accepted cumulative boundary", engine, err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if validationErr := session.Validate(strings.NewReader(`<r>text</r>`)); validationErr != nil {
				t.Fatal(validationErr)
			}
		})
	}
}
