package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
)

func TestBuiltinSimpleDeclarationsFollowValueIDs(t *testing.T) {
	t.Parallel()

	if BuiltinSimpleTypeCount() != int(value.BuiltinTypeCount) {
		t.Fatalf("BuiltinSimpleTypeCount() = %d, want %d", BuiltinSimpleTypeCount(), value.BuiltinTypeCount)
	}
	for id := range value.BuiltinTypeCount {
		name, ok := value.BuiltinTypeName(id)
		if !ok {
			t.Fatalf("value builtin %d has no name", id)
		}
		decl, ok := builtinSimpleDeclaration(id)
		if !ok {
			t.Fatalf("builtinSimpleDeclaration(%d) missing", id)
		}
		switch name {
		case "xml:lang":
			if decl.namespace != vocab.XMLNamespaceURI || decl.local != vocab.XMLAttrLang || decl.scope != DeclarationScopeNonGlobal {
				t.Fatalf("xml:lang declaration = {%q}%q scope %v", decl.namespace, decl.local, decl.scope)
			}
		case "xml:space":
			if decl.namespace != vocab.XMLNamespaceURI || decl.local != vocab.XMLAttrSpace || decl.scope != DeclarationScopeNonGlobal {
				t.Fatalf("xml:space declaration = {%q}%q scope %v", decl.namespace, decl.local, decl.scope)
			}
		default:
			if decl.namespace != vocab.XSDNamespaceURI || decl.local != name || decl.scope != DeclarationScopeGlobal {
				t.Fatalf("builtin %q declaration = {%q}%q scope %v", name, decl.namespace, decl.local, decl.scope)
			}
		}
	}
}

func TestBuiltinAttributeSeedsUseCanonicalValueIDs(t *testing.T) {
	t.Parallel()

	for i := range BuiltinAttributeCount() {
		seed, ok := BuiltinAttributeSeedAt(i)
		if !ok {
			t.Fatalf("BuiltinAttributeSeedAt(%d) missing", i)
		}
		local := seed.Local
		if seed.Namespace == vocab.XMLNamespaceURI && (local == vocab.XMLAttrLang || local == vocab.XMLAttrSpace) {
			local = "xml:" + local
		}
		valueName := map[string]string{
			vocab.XMLAttrBase:      vocab.XSDValueAnyURI,
			vocab.XMLAttrID:        vocab.XSDValueID,
			"xml:lang":             "xml:lang",
			"xml:space":            "xml:space",
			vocab.XLinkAttrType:    vocab.XSDValueString,
			vocab.XLinkAttrHref:    vocab.XSDValueAnyURI,
			vocab.XLinkAttrRole:    vocab.XSDValueAnyURI,
			vocab.XLinkAttrArcrole: vocab.XSDValueAnyURI,
			vocab.XLinkAttrTitle:   vocab.XSDValueString,
			vocab.XLinkAttrShow:    vocab.XSDValueString,
			vocab.XLinkAttrActuate: vocab.XSDValueString,
		}[local]
		want, ok := value.BuiltinTypeID(valueName)
		if !ok {
			t.Fatalf("seed %d references unknown value type for %s:%s", i, seed.Namespace, seed.Local)
		}
		got, ok := seed.TypeID(BuiltinIDs{})
		if !ok || got != want {
			t.Fatalf("seed %d type = %d, %v; want %d", i, got, ok, want)
		}
	}
}

func TestCanonicalBuiltinIDs(t *testing.T) {
	t.Parallel()

	got := canonicalBuiltinIDs(17)
	checks := map[string]SimpleTypeID{
		"anySimpleType": got.AnySimpleType,
		"string":        got.String,
		"boolean":       got.Boolean,
		"decimal":       got.Decimal,
		"integer":       got.Integer,
		"int":           got.Int,
		"date":          got.Date,
		"dateTime":      got.DateTime,
		"time":          got.Time,
		"anyURI":        got.AnyURI,
		"QName":         got.QName,
		"ID":            got.ID,
		"IDREF":         got.IDREF,
		"IDREFS":        got.IDREFS,
		"NMTOKEN":       got.NMTOKEN,
		"NMTOKENS":      got.NMTOKENS,
		"ENTITY":        got.ENTITY,
		"ENTITIES":      got.ENTITIES,
	}
	for name, id := range checks {
		want, ok := value.BuiltinTypeID(name)
		if !ok || id != want {
			t.Errorf("canonicalBuiltinIDs().%s = %d, want %d", name, id, want)
		}
	}
	if got.AnyType != 17 {
		t.Fatalf("AnyType = %d, want 17", got.AnyType)
	}
}

func TestValidateBuiltinDeclarationCounts(t *testing.T) {
	t.Parallel()

	valid := BuiltinDeclarationCounts{
		SimpleTypes:      BuiltinSimpleTypeCount(),
		Attributes:       BuiltinAttributeCount(),
		ComplexTypes:     BuiltinComplexTypeCount(),
		Wildcards:        1,
		AttributeUseSets: 1,
		Models:           1,
	}
	if err := ValidateBuiltinDeclarationCounts(valid); err != nil {
		t.Fatalf("ValidateBuiltinDeclarationCounts(valid) error = %v", err)
	}
	for name, mutate := range map[string]func(*BuiltinDeclarationCounts){
		"simple types":   func(c *BuiltinDeclarationCounts) { c.SimpleTypes-- },
		"attributes":     func(c *BuiltinDeclarationCounts) { c.Attributes-- },
		"complex types":  func(c *BuiltinDeclarationCounts) { c.ComplexTypes-- },
		"wildcards":      func(c *BuiltinDeclarationCounts) { c.Wildcards = 0 },
		"attribute sets": func(c *BuiltinDeclarationCounts) { c.AttributeUseSets = 0 },
		"models":         func(c *BuiltinDeclarationCounts) { c.Models = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			counts := valid
			mutate(&counts)
			if err := ValidateBuiltinDeclarationCounts(counts); err == nil {
				t.Fatal("ValidateBuiltinDeclarationCounts() unexpectedly succeeded")
			}
		})
	}
}
