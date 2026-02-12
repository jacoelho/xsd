package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestValidateAttributesNilSession(t *testing.T) {
	var sess *Session

	_, err := sess.ValidateAttributes(0, nil, nil)
	if err == nil {
		t.Fatalf("expected schema not loaded error")
	}
	if code, ok := validationErrorInfo(err); !ok || code != xsderrors.ErrSchemaNotLoaded {
		t.Fatalf("error code = %v, want %v", code, xsderrors.ErrSchemaNotLoaded)
	}
}

func TestValidateAttributesRequiredMissing(t *testing.T) {
	schema, ids := buildAttrFixture(t)
	sess := NewSession(schema)

	_, err := sess.ValidateAttributes(ids.typeBase, nil, nil)
	if err == nil {
		t.Fatalf("expected required attribute error")
	}
}

func TestValidateAttributesProhibited(t *testing.T) {
	schema, ids := buildAttrFixture(t)
	sess := NewSession(schema)

	attrs := []StartAttr{{Sym: ids.attrSymProhibited, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("prohib")}}
	_, err := sess.ValidateAttributes(ids.typeBase, attrs, nil)
	if err == nil {
		t.Fatalf("expected prohibited attribute error")
	}
}

func TestValidateAttributesDefaultApplied(t *testing.T) {
	schema, ids := buildAttrFixture(t)
	sess := NewSession(schema)

	_, err := sess.ValidateAttributes(ids.typeBase, nil, nil)
	if err == nil {
		t.Fatalf("expected required attribute error")
	}

	schema, ids = buildAttrFixtureNoRequired(t)
	sess = NewSession(schema)
	result, err := sess.ValidateAttributes(ids.typeBase, nil, nil)
	if err != nil {
		t.Fatalf("ValidateAttributes: %v", err)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("applied defaults = %d, want 1", len(result.Applied))
	}
	if result.Applied[0].Name != ids.attrSymDefault {
		t.Fatalf("default applied to %d, want %d", result.Applied[0].Name, ids.attrSymDefault)
	}
}

func TestValidateAttributesAllowsXMLNamespace(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	xmlAttrs := []StartAttr{{Sym: schema.Predef.XMLLang, NS: schema.PredefNS.XML, NSBytes: []byte("http://www.w3.org/XML/1998/namespace"), Local: []byte("lang")}}
	_, err := sess.ValidateAttributes(ids.typeBase, xmlAttrs, nil)
	if err != nil {
		t.Fatalf("expected xml attribute to be allowed, got %v", err)
	}
}

func TestValidateAttributesSimpleTypeXsiOnly(t *testing.T) {
	schema, ids := buildAttrFixture(t)
	sess := NewSession(schema)

	attrs := []StartAttr{{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")}}
	_, err := sess.ValidateAttributes(ids.typeSimple, attrs, nil)
	if err == nil {
		t.Fatalf("expected simple type attribute error")
	}

	xsiAttrs := []StartAttr{{Sym: schema.Predef.XsiType, NS: schema.PredefNS.Xsi, NSBytes: []byte("http://www.w3.org/2001/XMLSchema-instance"), Local: []byte("type")}}
	_, err = sess.ValidateAttributes(ids.typeSimple, xsiAttrs, nil)
	if err != nil {
		t.Fatalf("expected xsi attribute to be allowed")
	}

	xmlAttrs := []StartAttr{{Sym: schema.Predef.XMLLang, NS: schema.PredefNS.XML, NSBytes: []byte("http://www.w3.org/XML/1998/namespace"), Local: []byte("lang")}}
	_, err = sess.ValidateAttributes(ids.typeSimple, xmlAttrs, nil)
	if err != nil {
		t.Fatalf("expected xml attribute to be allowed")
	}
}

func TestValidateAttributesRejectsUnknownXsiAttribute(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	unknown := []StartAttr{{
		NS:      schema.PredefNS.Xsi,
		NSBytes: []byte("http://www.w3.org/2001/XMLSchema-instance"),
		Local:   []byte("unknown"),
		Value:   []byte("1"),
	}}
	if _, err := sess.ValidateAttributes(ids.typeBase, unknown, nil); err == nil {
		t.Fatalf("expected unknown xsi attribute error")
	}
	if _, err := sess.ValidateAttributes(ids.typeSimple, unknown, nil); err == nil {
		t.Fatalf("expected unknown xsi attribute error for simple type")
	}

	known := []StartAttr{{
		NS:      schema.PredefNS.Xsi,
		NSBytes: []byte("http://www.w3.org/2001/XMLSchema-instance"),
		Local:   []byte("nil"),
		Value:   []byte("true"),
	}}
	if _, err := sess.ValidateAttributes(ids.typeBase, known, nil); err != nil {
		t.Fatalf("expected xsi:nil to be allowed, got %v", err)
	}
}

func TestValidateAttributesWildcardStrictUnresolved(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	schema.ComplexTypes[1].AnyAttr = 1
	schema.Wildcards = []runtime.WildcardRule{
		{},
		{NS: runtime.NSConstraint{Kind: runtime.NSAny}, PC: runtime.PCStrict},
	}
	sess := NewSession(schema)

	attrs := []StartAttr{{Sym: 0, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("unknown")}}
	_, err := sess.ValidateAttributes(ids.typeBase, attrs, nil)
	if err == nil {
		t.Fatalf("expected strict wildcard error")
	}
}

func TestValidateAttributesWildcardLaxSkip(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	schema.ComplexTypes[1].AnyAttr = 1
	schema.Wildcards = []runtime.WildcardRule{
		{},
		{NS: runtime.NSConstraint{Kind: runtime.NSAny}, PC: runtime.PCLax},
	}
	sess := NewSession(schema)

	attrs := []StartAttr{{Sym: 0, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("unknown")}}
	_, err := sess.ValidateAttributes(ids.typeBase, attrs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttributesWildcardResolvesGlobal(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	schema.ComplexTypes[1].AnyAttr = 1
	schema.Wildcards = []runtime.WildcardRule{
		{},
		{NS: runtime.NSConstraint{Kind: runtime.NSAny}, PC: runtime.PCStrict},
	}
	sess := NewSession(schema)

	attrs := []StartAttr{{Sym: ids.attrSymGlobal, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("global")}}
	_, err := sess.ValidateAttributes(ids.typeBase, attrs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAttributesDuplicate(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	attrs := []StartAttr{{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")}, {Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")}}
	_, err := sess.ValidateAttributes(ids.typeBase, attrs, nil)
	if err == nil {
		t.Fatalf("expected duplicate attribute error")
	}
}

func TestValidateAttributesCopiesUncachedNamesWhenStored(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	schema.ICs = make([]runtime.IdentityConstraint, 2)
	sess := NewSession(schema)

	nsBuf := []byte("urn:test")
	localBuf := []byte("default")
	attrs := []StartAttr{{
		Sym:        ids.attrSymDefault,
		NS:         ids.nsID,
		NSBytes:    nsBuf,
		Local:      localBuf,
		NameCached: false,
	}}
	result, err := sess.ValidateAttributes(ids.typeBase, attrs, nil)
	if err != nil {
		t.Fatalf("ValidateAttributes: %v", err)
	}
	if len(result.Attrs) != 1 {
		t.Fatalf("stored attrs = %d, want 1", len(result.Attrs))
	}

	nsBuf[0] = 'x'
	localBuf[0] = 'x'
	if got := string(result.Attrs[0].NSBytes); got != "urn:test" {
		t.Fatalf("stored namespace = %q, want %q", got, "urn:test")
	}
	if got := string(result.Attrs[0].Local); got != "default" {
		t.Fatalf("stored local name = %q, want %q", got, "default")
	}
}

func buildAttrFixture(tb testing.TB) (*runtime.Schema, attrFixtureIDs) {
	tb.Helper()
	schema, ids := buildAttrFixtureNoRequired(tb)
	// add required attribute use
	schema.AttrIndex.Uses = append(schema.AttrIndex.Uses, runtime.AttrUse{Name: ids.attrSymRequired, Validator: 1, Use: runtime.AttrRequired})
	schema.ComplexTypes[1].Attrs.Len++
	return schema, ids
}

func buildAttrFixtureNoRequired(tb testing.TB) (*runtime.Schema, attrFixtureIDs) {
	tb.Helper()
	builder := runtime.NewBuilder()
	nsID := mustInternNamespace(tb, builder, []byte("urn:test"))
	xsiNS := mustInternNamespace(tb, builder, []byte("http://www.w3.org/2001/XMLSchema-instance"))
	mustInternSymbol(tb, builder, xsiNS, []byte("type"))
	mustInternSymbol(tb, builder, xsiNS, []byte("nil"))
	mustInternSymbol(tb, builder, xsiNS, []byte("schemaLocation"))
	mustInternSymbol(tb, builder, xsiNS, []byte("noNamespaceSchemaLocation"))

	attrDefault := mustInternSymbol(tb, builder, nsID, []byte("default"))
	attrProhib := mustInternSymbol(tb, builder, nsID, []byte("prohib"))
	attrGlobal := mustInternSymbol(tb, builder, nsID, []byte("global"))
	attrRequired := mustInternSymbol(tb, builder, nsID, []byte("required"))
	typeSym := mustInternSymbol(tb, builder, nsID, []byte("BaseType"))
	schema, err := builder.Build()
	if err != nil {
		tb.Fatalf("Build() error = %v", err)
	}
	schema.Validators = runtime.ValidatorsBundle{
		String: []runtime.StringValidator{{Kind: runtime.StringAny}},
		Meta: []runtime.ValidatorMeta{
			{},
			{
				Kind:       runtime.VString,
				Index:      0,
				WhiteSpace: runtime.WSPreserve,
			},
		},
	}

	schema.Types = make([]runtime.Type, 3)
	schema.Types[1] = runtime.Type{Name: typeSym, Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}}
	schema.Types[2] = runtime.Type{Name: 0, Kind: runtime.TypeSimple}
	schema.GlobalTypes = make([]runtime.TypeID, schema.Symbols.Count()+1)
	schema.GlobalTypes[typeSym] = 1

	schema.ComplexTypes = make([]runtime.ComplexType, 2)
	schema.AttrIndex = runtime.ComplexAttrIndex{Uses: []runtime.AttrUse{
		{Name: attrDefault, Validator: 1, Use: runtime.AttrOptional, Default: runtime.ValueRef{Present: true}},
		{Name: attrProhib, Validator: 1, Use: runtime.AttrProhibited},
	}}
	schema.ComplexTypes[1].Attrs = runtime.AttrIndexRef{Off: 0, Len: 2, Mode: runtime.AttrIndexSmallLinear}

	schema.Attributes = make([]runtime.Attribute, 2)
	schema.Attributes[1] = runtime.Attribute{Name: attrGlobal, Validator: 1}
	schema.GlobalAttributes = make([]runtime.AttrID, schema.Symbols.Count()+1)
	schema.GlobalAttributes[attrGlobal] = 1

	return schema, attrFixtureIDs{
		nsID:              nsID,
		attrSymDefault:    attrDefault,
		attrSymProhibited: attrProhib,
		attrSymGlobal:     attrGlobal,
		attrSymRequired:   attrRequired,
		typeBase:          1,
		typeSimple:        2,
	}
}

type attrFixtureIDs struct {
	nsID              runtime.NamespaceID
	attrSymDefault    runtime.SymbolID
	attrSymProhibited runtime.SymbolID
	attrSymGlobal     runtime.SymbolID
	attrSymRequired   runtime.SymbolID
	typeBase          runtime.TypeID
	typeSimple        runtime.TypeID
}
