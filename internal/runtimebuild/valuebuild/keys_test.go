package valuebuild

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func TestValueKeysForNormalizedUseRuntimeEncoding(t *testing.T) {
	intSpec := schemair.SimpleTypeSpec{
		TypeDecl:       1,
		Name:           schemair.Name{Local: "Int"},
		Primitive:      "decimal",
		BuiltinBase:    "int",
		Whitespace:     schemair.WhitespaceCollapse,
		IntegerDerived: true,
	}
	c := &artifactCompiler{
		simpleSpecs: map[schemair.TypeID]schemair.SimpleTypeSpec{
			1: intSpec,
			2: {
				TypeDecl:   2,
				Name:       schemair.Name{Local: "Boolean"},
				Primitive:  "boolean",
				Whitespace: schemair.WhitespaceCollapse,
			},
		},
	}

	tests := []struct {
		name       string
		lexical    string
		normalized string
		spec       schemair.SimpleTypeSpec
		ctx        map[string]string
		want       []runtime.ValueKey
	}{
		{
			name:       "decimal",
			lexical:    "1.0",
			normalized: "1.0",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "decimal",
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "decimal", "1.0", nil)},
		},
		{
			name:       "boolean",
			lexical:    "1",
			normalized: "1",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "boolean",
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "boolean", "1", nil)},
		},
		{
			name:       "string",
			lexical:    "abc",
			normalized: "abc",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "string",
				Whitespace: schemair.WhitespacePreserve,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "string", "abc", nil)},
		},
		{
			name:       "anyURI",
			lexical:    "urn:item",
			normalized: "urn:item",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "anyURI",
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "anyURI", "urn:item", nil)},
		},
		{
			name:       "hexBinary",
			lexical:    "0a0B",
			normalized: "0a0B",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "hexBinary",
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "hexBinary", "0a0B", nil)},
		},
		{
			name:       "base64Binary",
			lexical:    "AQID",
			normalized: "AQID",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "base64Binary",
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "base64Binary", "AQID", nil)},
		},
		{
			name:       "float",
			lexical:    "-0",
			normalized: "-0",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "float",
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "float", "-0", nil)},
		},
		{
			name:       "double",
			lexical:    "NaN",
			normalized: "NaN",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "double",
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "double", "NaN", nil)},
		},
		{
			name:       "duration",
			lexical:    "P1Y2M3DT4H5M6S",
			normalized: "P1Y2M3DT4H5M6S",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "duration",
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "duration", "P1Y2M3DT4H5M6S", nil)},
		},
		{
			name:       "dateTime",
			lexical:    "2024-01-02T03:04:05Z",
			normalized: "2024-01-02T03:04:05Z",
			spec: schemair.SimpleTypeSpec{
				Primitive:  "dateTime",
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "dateTime", "2024-01-02T03:04:05Z", nil)},
		},
		{
			name:       "list",
			lexical:    "01 1",
			normalized: "01 1",
			spec: schemair.SimpleTypeSpec{
				Variety:    schemair.TypeVarietyList,
				Item:       schemair.TypeRef{ID: 1, Name: schemair.Name{Local: "Int"}},
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{runtimeListKey(t,
				runtimeKeyForPrimitive(t, "decimal", "1", nil),
				runtimeKeyForPrimitive(t, "decimal", "1", nil),
			)},
		},
		{
			name:       "union",
			lexical:    "1",
			normalized: "1",
			spec: schemair.SimpleTypeSpec{
				Variety: schemair.TypeVarietyUnion,
				Members: []schemair.TypeRef{
					{ID: 2, Name: schemair.Name{Local: "Boolean"}},
					{ID: 1, Name: schemair.Name{Local: "Int"}},
				},
				Whitespace: schemair.WhitespaceCollapse,
			},
			want: []runtime.ValueKey{
				runtimeKeyForPrimitive(t, "boolean", "1", nil),
				runtimeKeyForPrimitive(t, "decimal", "1", nil),
			},
		},
		{
			name:       "QName",
			lexical:    "p:item",
			normalized: "p:item",
			spec: schemair.SimpleTypeSpec{
				Primitive:       "QName",
				Whitespace:      schemair.WhitespaceCollapse,
				QNameOrNotation: true,
			},
			ctx:  map[string]string{"p": "urn:q"},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "QName", "p:item", map[string]string{"p": "urn:q"})},
		},
		{
			name:       "NOTATION",
			lexical:    "p:item",
			normalized: "p:item",
			spec: schemair.SimpleTypeSpec{
				Primitive:       "NOTATION",
				Whitespace:      schemair.WhitespaceCollapse,
				QNameOrNotation: true,
			},
			ctx:  map[string]string{"p": "urn:q"},
			want: []runtime.ValueKey{runtimeKeyForPrimitive(t, "NOTATION", "p:item", map[string]string{"p": "urn:q"})},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.valueKeysForNormalized(tt.lexical, tt.normalized, tt.spec, tt.ctx)
			if err != nil {
				t.Fatalf("valueKeysForNormalized() error = %v", err)
			}
			assertRuntimeKeysEqual(t, got, tt.want)
		})
	}
}

func TestValidateEnumSetsAcceptsEquivalentLexicalForms(t *testing.T) {
	spec := schemair.SimpleTypeSpec{
		TypeDecl:   1,
		Name:       schemair.Name{Local: "EnumDecimal"},
		Primitive:  "decimal",
		Whitespace: schemair.WhitespaceCollapse,
		Facets: []schemair.FacetSpec{{
			Kind: schemair.FacetEnumeration,
			Name: "enumeration",
			Values: []schemair.FacetValue{{
				Lexical: "1.0",
			}},
		}},
	}
	c, err := newArtifactCompiler(&schemair.Schema{
		SimpleTypes: []schemair.SimpleTypeSpec{spec},
	})
	if err != nil {
		t.Fatalf("newArtifactCompiler() error = %v", err)
	}

	if err := c.validateEnumSets("1", "1", spec, nil); err != nil {
		t.Fatalf("validateEnumSets() error = %v", err)
	}
}

func runtimeKeyForPrimitive(t *testing.T, primitive, normalized string, ctx map[string]string) runtime.ValueKey {
	t.Helper()
	kind, bytes, err := runtime.KeyForPrimitiveName(primitive, normalized, ctx)
	if err != nil {
		t.Fatalf("KeyForPrimitiveName(%s, %s) error = %v", primitive, normalized, err)
	}
	return runtime.ValueKey{
		Kind:  kind,
		Bytes: bytes,
		Hash:  runtime.HashKey(kind, bytes),
	}
}

func runtimeListKey(t *testing.T, items ...runtime.ValueKey) runtime.ValueKey {
	t.Helper()
	buf := runtime.AppendUvarint(nil, uint64(len(items)))
	for _, item := range items {
		buf = runtime.AppendListEntry(buf, byte(item.Kind), item.Bytes)
	}
	return runtime.ValueKey{
		Kind:  runtime.VKList,
		Bytes: buf,
		Hash:  runtime.HashKey(runtime.VKList, buf),
	}
}

func assertRuntimeKeysEqual(t *testing.T, got, want []runtime.ValueKey) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("key count = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i].Kind != want[i].Kind || got[i].Hash != want[i].Hash || !bytes.Equal(got[i].Bytes, want[i].Bytes) {
			t.Fatalf("key[%d] = {kind:%d hash:%d bytes:%v}, want {kind:%d hash:%d bytes:%v}",
				i,
				got[i].Kind,
				got[i].Hash,
				got[i].Bytes,
				want[i].Kind,
				want[i].Hash,
				want[i].Bytes,
			)
		}
	}
}
