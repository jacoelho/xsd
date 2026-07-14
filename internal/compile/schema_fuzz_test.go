package compile_test

import (
	"context"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/source"
)

func FuzzSchemaParserLimits(f *testing.F) {
	for _, seed := range []string{
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`,
		`<!DOCTYPE xs:schema><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="a" name="b"/></xs:schema>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:appinfo><q:payload/></xs:appinfo></xs:annotation></xs:schema>`,
		`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="v" type="xs:int"/></xs:sequence></xs:complexType></xs:element></xs:schema>`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, schema string) {
		if len(schema) > 8192 {
			t.Skip()
		}
		if _, err := compile.Compile(context.Background(),
			compile.Options{
				MaxSchemaDepth:        32,
				MaxSchemaAttributes:   32,
				MaxSchemaTokenBytes:   4096,
				MaxSchemaSourceBytes:  8192,
				MaxSchemaNames:        256,
				MaxFiniteOccurs:       256,
				MaxContentModelStates: 256,
			},
			[]source.Source{
				source.Bytes("fuzz.xsd", []byte(schema)),
			}); err != nil {
			return
		}
	})
}
