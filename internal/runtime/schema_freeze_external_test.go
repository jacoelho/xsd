package runtime_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestSimpleTypeFreezeProjectionsCloneUnionSlices(t *testing.T) {
	t.Parallel()

	st := runtime.SimpleType{
		Union:   []runtime.SimpleTypeID{1},
		Variety: runtime.SimpleVarietyUnion,
	}
	restriction := runtime.NewSimpleTypeRestrictionValidationForSimpleType(st)
	restriction.Union[0] = 9
	if st.Union[0] != 1 {
		t.Fatalf("NewSimpleTypeRestrictionValidationForSimpleType returned table-backed union slice: %#v", st.Union)
	}

	simpleTypes := []runtime.SimpleType{st}
	nodes := runtime.NewSimpleTypeGraphNodesForSimpleTypes(simpleTypes)
	nodes[0].Union[0] = 9
	if simpleTypes[0].Union[0] != 1 {
		t.Fatalf("NewSimpleTypeGraphNodesForSimpleTypes returned table-backed union slice: %#v", simpleTypes[0].Union)
	}
}

func TestValueConstraintIdentityClonesResolvedNames(t *testing.T) {
	t.Parallel()

	vc := &runtime.ValueConstraint{
		ResolvedNames: []runtime.ResolvedValueName{{Lexical: "p:item"}},
	}
	identity := runtime.NewValueConstraintIdentity(vc)
	identity.ResolvedNames[0].Lexical = "p:other"
	if vc.ResolvedNames[0].Lexical != "p:item" {
		t.Fatalf("NewValueConstraintIdentity returned table-backed resolved names: %#v", vc.ResolvedNames)
	}
}

func TestRuntimeGlobalsClonesMapAndNameProjectionState(t *testing.T) {
	t.Parallel()

	attrName := runtime.QName{Local: 1}
	elemName := runtime.QName{Local: 2}
	simpleName := runtime.QName{Local: 3}
	complexName := runtime.QName{Local: 4}
	identityName := runtime.QName{Local: 5}
	notationName := runtime.QName{Local: 6}
	replacement := runtime.QName{Local: 99}
	build := runtime.SchemaBuild{
		GlobalAttributes: map[runtime.QName]runtime.AttributeID{attrName: 0},
		GlobalElements:   map[runtime.QName]runtime.ElementID{elemName: 0},
		GlobalTypes:      map[runtime.QName]runtime.TypeID{simpleName: runtime.SimpleRef(0), complexName: runtime.ComplexRef(0)},
		GlobalIdentities: map[runtime.QName]runtime.IdentityConstraintID{identityName: 0},
		Notations:        map[runtime.QName]bool{notationName: true},
		Attributes:       []runtime.AttributeDecl{{Name: attrName}},
		Elements:         []runtime.ElementDecl{{Name: elemName}},
		SimpleTypes:      []runtime.SimpleType{{Name: simpleName}},
		ComplexTypes:     []runtime.ComplexType{{Name: complexName}},
		Identities:       []runtime.IdentityConstraint{{Name: identityName}},
	}

	globals := build.RuntimeGlobals()
	globals.GlobalAttributes[attrName] = 9
	globals.GlobalElements[elemName] = 9
	globals.GlobalTypes[simpleName] = runtime.ComplexRef(9)
	globals.GlobalIdentities[identityName] = 9
	globals.Notations[notationName] = false
	globals.AttributeNames[0] = replacement
	globals.ElementNames[0] = replacement
	globals.SimpleTypeNames[0] = replacement
	globals.ComplexTypeNames[0] = replacement
	globals.IdentityNames[0] = replacement

	if build.GlobalAttributes[attrName] != 0 ||
		build.GlobalElements[elemName] != 0 ||
		build.GlobalTypes[simpleName] != runtime.SimpleRef(0) ||
		build.GlobalIdentities[identityName] != 0 ||
		!build.Notations[notationName] ||
		build.Attributes[0].Name != attrName ||
		build.Elements[0].Name != elemName ||
		build.SimpleTypes[0].Name != simpleName ||
		build.ComplexTypes[0].Name != complexName ||
		build.Identities[0].Name != identityName {
		t.Fatalf("runtimeGlobals returned table-backed projection state: %#v", build)
	}
}
