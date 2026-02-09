package validator

import (
	"slices"
	"strconv"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func Benchmark_QNameValueParsing_PrefixHeavy(b *testing.B) {
	const prefixCount = 32
	local := []byte("value")

	decls := make([]xmlstream.NamespaceDecl, prefixCount)
	qnames := make([][]byte, prefixCount)
	maxLen := 0
	for i := range prefixCount {
		prefix := make([]byte, 0, 8)
		prefix = append(prefix, 'p')
		prefix = strconv.AppendInt(prefix, int64(i), 10)
		uri := "urn:test:" + strconv.Itoa(i)
		decls[i] = xmlstream.NamespaceDecl{Prefix: string(prefix), URI: uri}

		qname := make([]byte, 0, len(prefix)+1+len(local))
		qname = append(qname, prefix...)
		qname = append(qname, ':')
		qname = append(qname, local...)
		qnames[i] = qname
		if len(uri)+1+len(local) > maxLen {
			maxLen = len(uri) + 1 + len(local)
		}
	}

	schema, err := runtime.NewBuilder().Build()
	if err != nil {
		b.Fatalf("Build() error = %v", err)
	}
	sess := NewSession(schema)
	sess.pushNamespaceScope(slices.Values(decls))
	resolver := sessionResolver{s: sess}

	dst := make([]byte, 0, maxLen)
	for _, qname := range qnames {
		if _, err := value.CanonicalQName(qname, resolver, dst[:0]); err != nil {
			b.Fatalf("canonical QName: %v", err)
		}
	}

	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		qname := qnames[i%len(qnames)]
		if _, err := value.CanonicalQName(qname, resolver, dst[:0]); err != nil {
			b.Fatalf("canonical QName: %v", err)
		}
	}
	b.StopTimer()

	sess.popNamespaceScope()
}

func Benchmark_SessionShrinkPolicy(b *testing.B) {
	sess := &Session{}
	large := make([]byte, maxSessionBuffer+1024)
	small := []byte("ok")

	b.ReportAllocs()
	for b.Loop() {
		sess.textBuf = append(sess.textBuf[:0], large...)
		sess.Reset()
		sess.textBuf = append(sess.textBuf[:0], small...)
		sess.Reset()
	}
}

func Benchmark_UnionValidation_Overlap(b *testing.B) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:decimal"/>
  </xs:simpleType>
</xs:schema>`

	rt := mustBuildRuntimeSchema(b, schema)
	sess := NewSession(rt)
	sym := rt.Symbols.Lookup(rt.PredefNS.Empty, []byte("U"))
	typeID := rt.GlobalTypes[sym]
	if typeID == 0 {
		b.Fatalf("union type not found")
	}
	validator := rt.Types[typeID].Validator
	input := []byte("12.5")
	opts := valueOptions{applyWhitespace: true}

	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := sess.validateValueInternalWithMetrics(validator, input, nil, opts); err != nil {
			b.Fatalf("validate union: %v", err)
		}
	}
}

func Benchmark_EnumLookup_TypedKeys(b *testing.B) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="U">
    <xs:union memberTypes="xs:int xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="E">
    <xs:restriction base="U">
      <xs:enumeration value="1"/>
      <xs:enumeration value="one"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`

	rt := mustBuildRuntimeSchema(b, schema)
	sess := NewSession(rt)
	sym := rt.Symbols.Lookup(rt.PredefNS.Empty, []byte("E"))
	typeID := rt.GlobalTypes[sym]
	if typeID == 0 {
		b.Fatalf("enum type not found")
	}
	validator := rt.Types[typeID].Validator
	meta := rt.Validators.Meta[validator]
	start := int(meta.Facets.Off)
	end := start + int(meta.Facets.Len)
	enumID := runtime.EnumID(0)
	for _, instr := range rt.Facets[start:end] {
		if instr.Op == runtime.FEnum {
			enumID = runtime.EnumID(instr.Arg0)
			break
		}
	}
	if enumID == 0 {
		b.Fatalf("enum facet missing")
	}
	_, metrics, err := sess.validateValueInternalWithMetrics(validator, []byte("one"), nil, valueOptions{applyWhitespace: true})
	if err != nil {
		b.Fatalf("enum validate: %v", err)
	}
	key := append([]byte(nil), metrics.keyBytes...)
	kind := metrics.keyKind

	b.ReportAllocs()
	for b.Loop() {
		if !runtime.EnumContains(&rt.Enums, enumID, kind, key) {
			b.Fatalf("enum lookup failed")
		}
	}
}
