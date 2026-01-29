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
	if code, _, ok := validationErrorInfo(err); !ok || code != xsderrors.ErrSchemaNotLoaded {
		t.Fatalf("error code = %v, want %v", code, xsderrors.ErrSchemaNotLoaded)
	}
}

func TestValidateAttributesRequiredMissing(t *testing.T) {
	schema, ids := buildAttrFixture()
	sess := NewSession(schema)

	_, err := sess.ValidateAttributes(ids.typeBase, nil, nil)
	if err == nil {
		t.Fatalf("expected required attribute error")
	}
}

func TestValidateAttributesProhibited(t *testing.T) {
	schema, ids := buildAttrFixture()
	sess := NewSession(schema)

	attrs := []StartAttr{{Sym: ids.attrSymProhibited, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("prohib")}}
	_, err := sess.ValidateAttributes(ids.typeBase, attrs, nil)
	if err == nil {
		t.Fatalf("expected prohibited attribute error")
	}
}

func TestValidateAttributesDefaultApplied(t *testing.T) {
	schema, ids := buildAttrFixture()
	sess := NewSession(schema)

	_, err := sess.ValidateAttributes(ids.typeBase, nil, nil)
	if err == nil {
		t.Fatalf("expected required attribute error")
	}

	schema, ids = buildAttrFixtureNoRequired()
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

func TestValidateAttributesSimpleTypeXsiOnly(t *testing.T) {
	schema, ids := buildAttrFixture()
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
}

func TestValidateAttributesWildcardStrictUnresolved(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired()
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
	schema, ids := buildAttrFixtureNoRequired()
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
	schema, ids := buildAttrFixtureNoRequired()
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
	schema, ids := buildAttrFixtureNoRequired()
	sess := NewSession(schema)

	attrs := []StartAttr{{Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")}, {Sym: ids.attrSymDefault, NS: ids.nsID, NSBytes: []byte("urn:test"), Local: []byte("default")}}
	_, err := sess.ValidateAttributes(ids.typeBase, attrs, nil)
	if err == nil {
		t.Fatalf("expected duplicate attribute error")
	}
}

func buildAttrFixture() (*runtime.Schema, attrFixtureIDs) {
	schema, ids := buildAttrFixtureNoRequired()
	// add required attribute use
	schema.AttrIndex.Uses = append(schema.AttrIndex.Uses, runtime.AttrUse{Name: ids.attrSymRequired, Use: runtime.AttrRequired})
	schema.ComplexTypes[1].Attrs.Len++
	return schema, ids
}

func buildAttrFixtureNoRequired() (*runtime.Schema, attrFixtureIDs) {
	builder := runtime.NewBuilder()
	nsID := builder.InternNamespace([]byte("urn:test"))
	xsiNS := builder.InternNamespace([]byte("http://www.w3.org/2001/XMLSchema-instance"))
	builder.InternSymbol(xsiNS, []byte("type"))
	builder.InternSymbol(xsiNS, []byte("nil"))
	builder.InternSymbol(xsiNS, []byte("schemaLocation"))
	builder.InternSymbol(xsiNS, []byte("noNamespaceSchemaLocation"))

	attrDefault := builder.InternSymbol(nsID, []byte("default"))
	attrProhib := builder.InternSymbol(nsID, []byte("prohib"))
	attrGlobal := builder.InternSymbol(nsID, []byte("global"))
	attrRequired := builder.InternSymbol(nsID, []byte("required"))
	typeSym := builder.InternSymbol(nsID, []byte("BaseType"))
	schema := builder.Build()
	schema.Validators = runtime.ValidatorsBundle{
		String: []runtime.StringValidator{{Kind: runtime.StringAny}},
		Meta: []runtime.ValidatorMeta{{
			Kind:       runtime.VString,
			Index:      0,
			WhiteSpace: runtime.WS_Preserve,
		}},
	}

	schema.Types = make([]runtime.Type, 3)
	schema.Types[1] = runtime.Type{Name: typeSym, Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}}
	schema.Types[2] = runtime.Type{Name: 0, Kind: runtime.TypeSimple}
	schema.GlobalTypes = make([]runtime.TypeID, schema.Symbols.Count()+1)
	schema.GlobalTypes[typeSym] = 1

	schema.ComplexTypes = make([]runtime.ComplexType, 2)
	schema.AttrIndex = runtime.ComplexAttrIndex{Uses: []runtime.AttrUse{
		{Name: attrDefault, Use: runtime.AttrOptional, Default: runtime.ValueRef{Present: true}},
		{Name: attrProhib, Use: runtime.AttrProhibited},
	}}
	schema.ComplexTypes[1].Attrs = runtime.AttrIndexRef{Off: 0, Len: 2, Mode: runtime.AttrIndexSmallLinear}

	schema.Attributes = make([]runtime.Attribute, 2)
	schema.Attributes[1] = runtime.Attribute{Name: attrGlobal}
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
