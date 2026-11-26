package schema

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
)

func TestIndexedRowsDispatchAsLinearScan(t *testing.T) {
	t.Parallel()

	optional := func(b *strings.Builder, wildcardNamespace string) {
		b.WriteString(`<xs:element ref="head" minOccurs="0"/>`)
		for i := range 9 {
			fmt.Fprintf(b, `<xs:element name="e%d" type="xs:string" minOccurs="0"/>`, i)
		}
		if wildcardNamespace != "" {
			fmt.Fprintf(b, `<xs:any namespace="%s" processContents="skip" minOccurs="0"/>`, wildcardNamespace)
		}
		b.WriteString(`<xs:element name="c" type="xs:string" minOccurs="0" maxOccurs="3"/>`)
	}
	wildcardNamespaces := []string{"urn:w1", "urn:w2"}
	for _, test := range []struct {
		name    string
		content func(*strings.Builder)
		kind    publishedDFAIndexKind
	}{
		{
			name: "names repeated around a required element, with wildcards",
			content: func(b *strings.Builder) {
				optional(b, "urn:w1")
				b.WriteString(`<xs:element name="x" type="xs:string"/>`)
				optional(b, "urn:w2")
			},
			kind: publishedDFARun,
		},
		{
			name: "distinct names of one namespace",
			content: func(b *strings.Builder) {
				optional(b, "")
				b.WriteString(`<xs:element name="x" type="xs:string"/>`)
				for i := range 9 {
					fmt.Fprintf(b, `<xs:element name="f%d" type="xs:string" minOccurs="0"/>`, i)
				}
			},
			kind: publishedDFARunDense,
		},
		{
			name: "repeated choice of names and wildcards",
			content: func(b *strings.Builder) {
				b.WriteString(`<xs:choice minOccurs="0" maxOccurs="3">`)
				optional(b, "urn:w1")
				b.WriteString(`<xs:any namespace="urn:w2" processContents="lax"/></xs:choice>`)
			},
			kind: publishedDFAMap,
		},
		{
			name: "repeated choice of sixteen names",
			content: func(b *strings.Builder) {
				b.WriteString(`<xs:choice minOccurs="2" maxOccurs="3"><xs:element ref="head"/>`)
				for i := range 15 {
					fmt.Fprintf(b, `<xs:element name="f%d" type="xs:string"/>`, i)
				}
				b.WriteString(`</xs:choice>`)
			},
			kind: publishedDFADense,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var b strings.Builder
			b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` +
				`<xs:element name="head" type="xs:string"/>` +
				`<xs:element name="member" type="xs:string" substitutionGroup="head"/>` +
				`<xs:element name="root"><xs:complexType><xs:sequence>`)
			test.content(&b)
			b.WriteString(`</xs:sequence></xs:complexType></xs:element></xs:schema>`)
			sources := []source.Source{source.Bytes("schema.xsd", []byte(b.String()))}
			for _, namespace := range wildcardNamespaces {
				sources = append(sources, source.Bytes(namespace+".xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="`+namespace+`">`+
					`<xs:element name="c" type="xs:string"/><xs:element name="e0" type="xs:string"/></xs:schema>`)))
			}
			rt, err := compileSources(Options{}, sources)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			inputs := []ContentInput{{Name: RuntimeName{}}}
			for _, local := range []string{"head", "member", "e0", "e4", "e8", "c", "x", "f0", "f8", "f14", "root"} {
				name, ok := rt.LookupQName("", local)
				if !ok {
					continue
				}
				inputs = append(inputs, ContentInput{Name: RuntimeName{known: true, name: name, local: local}})
			}
			for _, namespace := range wildcardNamespaces {
				inputs = append(inputs, ContentInput{Name: RuntimeName{ns: namespace, local: "any"}})
				for _, local := range []string{"c", "e0"} {
					name, ok := rt.LookupQName(namespace, local)
					if !ok {
						t.Fatalf("LookupQName(%q, %q) is unknown", namespace, local)
					}
					inputs = append(inputs, ContentInput{Name: RuntimeName{ns: namespace, known: true, name: name, local: local}})
				}
			}
			indexed := 0
			for id := range rt.program.CompiledModels {
				model := &rt.program.CompiledModels[id]
				linear := *model
				linear.Rows = slices.Clone(model.Rows)
				for i := range linear.Rows {
					if linear.Rows[i].indexKind == test.kind {
						indexed++
					}
					linear.Rows[i].index = publishedDFAIndex{}
					linear.Rows[i].indexKind = publishedDFALinear
				}
				for state := range model.Rows {
					for count := range uint32(4) {
						st := ContentState{model: contentModelID(id), state: uint32(state), count: count, present: true}
						for _, in := range inputs {
							gotTransition, gotStatus := rt.nextPublishedDFAContent(st, model, in)
							wantTransition, wantStatus := rt.nextPublishedDFAContent(st, &linear, in)
							if gotStatus != wantStatus || !reflect.DeepEqual(gotTransition, wantTransition) {
								t.Fatalf("model %d state %d count %d input %+v: indexed %+v/%v, linear %+v/%v", id, state, count, in.Name, gotTransition, gotStatus, wantTransition, wantStatus)
							}
						}
					}
				}
			}
			if indexed < 3 {
				t.Fatalf("schema has %d rows of index kind %d; the fixture no longer exercises it", indexed, test.kind)
			}
		})
	}
}
