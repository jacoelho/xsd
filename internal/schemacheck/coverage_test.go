package schemacheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestAttributeGroupUniqueness(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:attr"
	schema.AttributeFormDefault = parser.Qualified

	attr1 := &types.AttributeDecl{Name: types.QName{Local: "dup"}, Form: types.FormDefault}
	attr2 := &types.AttributeDecl{Name: types.QName{Local: "dup"}, Form: types.FormDefault}
	ag := &types.AttributeGroup{
		Name:       types.QName{Namespace: "urn:attr", Local: "ag"},
		Attributes: []*types.AttributeDecl{attr1, attr2},
	}

	if err := validateAttributeGroupUniqueness(schema, ag); err == nil {
		t.Fatalf("expected duplicate attribute error")
	}

	ref := &types.AttributeDecl{
		Name:        types.QName{Namespace: "urn:attr", Local: "ref"},
		IsReference: true,
	}
	if got := effectiveAttributeQNameForValidation(schema, ref); got != ref.Name {
		t.Fatalf("expected reference attribute QName to remain unchanged")
	}
}

func TestCollectEffectiveAttributeUses(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:attr"
	schema.AttributeFormDefault = parser.Qualified

	stringType := types.GetBuiltin(types.TypeName("string"))
	baseAttr := &types.AttributeDecl{
		Name: types.QName{Local: "base"},
		Type: stringType,
		Use:  types.Required,
	}

	baseCT := types.NewComplexType(types.QName{Namespace: "urn:attr", Local: "baseType"}, "urn:attr")
	baseCT.SetAttributes([]*types.AttributeDecl{baseAttr})
	baseCT.SetContent(&types.EmptyContent{})

	prohibit := &types.AttributeDecl{Name: types.QName{Local: "base"}, Use: types.Prohibited}
	agQName := types.QName{Namespace: "urn:attr", Local: "ag"}
	schema.AttributeGroups[agQName] = &types.AttributeGroup{
		Name:       agQName,
		Attributes: []*types.AttributeDecl{prohibit},
	}

	derivedAttr := &types.AttributeDecl{Name: types.QName{Local: "derived"}, Type: stringType}
	derivedCT := types.NewComplexType(types.QName{Namespace: "urn:attr", Local: "derivedType"}, "urn:attr")
	derivedCT.ResolvedBase = baseCT
	derivedCT.SetAttributes([]*types.AttributeDecl{derivedAttr})
	derivedCT.AttrGroups = []types.QName{agQName}
	derivedCT.SetContent(&types.EmptyContent{})

	attrMap := collectEffectiveAttributeUses(schema, derivedCT)
	if _, ok := attrMap[types.QName{Namespace: "urn:attr", Local: "base"}]; ok {
		t.Fatalf("expected prohibited attribute to be removed")
	}
	if _, ok := attrMap[types.QName{Namespace: "urn:attr", Local: "derived"}]; !ok {
		t.Fatalf("expected derived attribute to be present")
	}
}

func TestValidateRestrictionAttributes(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:attr"
	schema.AttributeFormDefault = parser.Qualified

	stringType := types.GetBuiltin(types.TypeName("string"))
	baseAttr := &types.AttributeDecl{
		Name:     types.QName{Namespace: "urn:attr", Local: "a"},
		Type:     stringType,
		Use:      types.Required,
		Fixed:    "fixed",
		HasFixed: true,
	}
	baseCT := types.NewComplexType(types.QName{Namespace: "urn:attr", Local: "base"}, "urn:attr")
	baseCT.SetAttributes([]*types.AttributeDecl{baseAttr})
	baseCT.SetContent(&types.EmptyContent{})
	schema.AttributeDecls[baseAttr.Name] = baseAttr

	restrictionAttr := &types.AttributeDecl{
		Name:        baseAttr.Name,
		IsReference: true,
		Use:         types.Optional,
	}

	if err := validateRestrictionAttributes(schema, baseCT, []*types.AttributeDecl{restrictionAttr}, "restriction"); err == nil {
		t.Fatalf("expected restriction attribute validation error")
	}
}

func TestSimpleAndComplexContentStructure(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:content"

	simpleBase := &types.SimpleType{
		QName:       types.QName{Namespace: "urn:content", Local: "simpleBase"},
		Restriction: &types.Restriction{Base: types.QName{Namespace: types.XSDNamespace, Local: "string"}},
	}
	schema.TypeDefs[simpleBase.QName] = simpleBase

	cc := &types.ComplexContent{
		Extension: &types.Extension{Base: simpleBase.QName},
	}
	if err := validateComplexContentStructure(schema, cc); err == nil {
		t.Fatalf("expected complexContent extension to reject simpleType base")
	}

	baseCT := types.NewComplexType(types.QName{Namespace: "urn:content", Local: "baseCT"}, "urn:content")
	baseCT.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: 1,
			MaxOccurs: 1,
		},
	})
	schema.TypeDefs[baseCT.QName] = baseCT

	sc := &types.SimpleContent{
		Restriction: &types.Restriction{Base: baseCT.QName},
	}
	if err := validateSimpleContentStructure(schema, sc, false); err == nil {
		t.Fatalf("expected simpleContent restriction to reject non-simpleContent base")
	}
}

func TestFacetListAndEnumeration(t *testing.T) {
	itemType := types.GetBuiltin(types.TypeName("NCName"))
	listType := types.NewSimpleType(types.QName{Namespace: "urn:facets", Local: "list"}, "urn:facets")
	listType.SetVariety(types.ListVariety)
	listType.ItemType = itemType

	enumFacet := &types.Enumeration{Values: []string{"alpha beta"}}
	if err := validateEnumerationValues([]types.Facet{enumFacet}, listType); err != nil {
		t.Fatalf("validateEnumerationValues error = %v", err)
	}

	emptyFacet := &types.Enumeration{Values: []string{""}}
	if err := validateEnumerationValues([]types.Facet{emptyFacet}, listType); err == nil {
		t.Fatalf("expected enumeration list item error")
	}

	if !isListTypeForFacets(listType, types.QName{}) {
		t.Fatalf("expected list type to be recognized")
	}
	if !isListTypeForFacets(nil, types.QName{Namespace: types.XSDNamespace, Local: "IDREFS"}) {
		t.Fatalf("expected built-in list type to be recognized")
	}

	complexType := types.NewComplexType(types.QName{Namespace: "urn:facets", Local: "ct"}, "urn:facets")
	if err := validateListItemValue(complexType, "x"); err == nil {
		t.Fatalf("expected list item validation to reject non-simple type")
	}
}

func TestFacetDateTimeHelpers(t *testing.T) {
	date, hasTZ, err := parseXSDDate("2001-10-26Z")
	if err != nil || !hasTZ {
		t.Fatalf("parseXSDDate error = %v", err)
	}
	date2, hasTZ, err := parseXSDDate("2001-10-26+01:00")
	if err != nil || !hasTZ {
		t.Fatalf("parseXSDDate error = %v", err)
	}
	if compareTimes(date, date2) == 0 {
		t.Fatalf("expected dates with different offsets to compare non-zero")
	}

	if _, _, err := parseXSDDateTime("2001-10-26T21:32:52Z"); err != nil {
		t.Fatalf("parseXSDDateTime error = %v", err)
	}
	if _, _, err := parseXSDTime("13:20:00+02:00"); err != nil {
		t.Fatalf("parseXSDTime error = %v", err)
	}
	if _, _, _, err := splitTimezone("2001-01-01+aa:bb"); err == nil {
		t.Fatalf("expected splitTimezone error")
	}

	if cmp, err := compareDurationValues("P1Y", "P2Y"); err != nil || cmp >= 0 {
		t.Fatalf("expected duration compare to order values")
	}
	if _, err := compareDurationValues("P1M", "P30D"); err == nil {
		t.Fatalf("expected duration compare to be not comparable")
	}
	if _, _, err := parseDurationParts("bad"); err == nil {
		t.Fatalf("expected parseDurationParts error")
	}
}

func TestFieldResolutionHelpers(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:field"

	stringType := types.GetBuiltin(types.TypeName("string"))
	grand := &types.ElementDecl{
		Name:      types.QName{Local: "grand"},
		Type:      stringType,
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	childType := types.NewComplexType(types.QName{Namespace: "urn:field", Local: "childType"}, "urn:field")
	childType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: 1,
			MaxOccurs: 1,
			Particles: []types.Particle{grand},
		},
	})

	child := &types.ElementDecl{
		Name:      types.QName{Local: "child"},
		Type:      childType,
		MinOccurs: 1,
		MaxOccurs: 1,
	}

	rootType := types.NewComplexType(types.QName{Namespace: "urn:field", Local: "rootType"}, "urn:field")
	rootType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: 1,
			MaxOccurs: 1,
			Particles: []types.Particle{child},
		},
	})

	attr := &types.AttributeDecl{Name: types.QName{Local: "id"}, Type: stringType}
	agQName := types.QName{Namespace: "urn:field", Local: "ag"}
	schema.AttributeGroups[agQName] = &types.AttributeGroup{
		Name:       agQName,
		Attributes: []*types.AttributeDecl{attr},
	}
	rootType.AttrGroups = []types.QName{agQName}

	root := &types.ElementDecl{
		Name:      types.QName{Local: "root"},
		Type:      rootType,
		MinOccurs: 1,
		MaxOccurs: 1,
	}

	if got, err := findElementTypeDescendant(schema, root, "grand"); err != nil || got == nil {
		t.Fatalf("findElementTypeDescendant error = %v", err)
	}

	if got, err := findAttributeType(schema, root, "id"); err != nil || got == nil {
		t.Fatalf("findAttributeType error = %v", err)
	}

	if got, err := resolveFieldElementType(schema, root, "child/grand"); err != nil || got == nil {
		t.Fatalf("resolveFieldElementType error = %v", err)
	}
}

func TestParticleHelpers(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:particles"

	stringType := types.GetBuiltin(types.TypeName("string"))
	elemA := &types.ElementDecl{
		Name:      types.QName{Local: "a"},
		Type:      stringType,
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	elemB := &types.ElementDecl{
		Name:      types.QName{Local: "b"},
		Type:      stringType,
		MinOccurs: 1,
		MaxOccurs: 1,
	}

	nested := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemB},
	}
	seq := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA, nested},
	}

	if got := normalizePointlessParticle(seq); got != seq {
		t.Fatalf("expected normalizePointlessParticle to keep multi-child group")
	}

	children := derivationChildren(seq)
	if len(children) != 2 {
		t.Fatalf("expected derivationChildren to flatten nested sequence")
	}

	minOcc, maxOcc := calculateEffectiveOccurrence(seq)
	if minOcc != 2 || maxOcc != 2 {
		t.Fatalf("unexpected effective occurrence: %d..%d", minOcc, maxOcc)
	}

	choice := &types.ModelGroup{
		Kind:      types.Choice,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA, elemB},
	}
	if err := validateParticleRestrictionWithKindChange(schema, choice, &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA},
	}); err != nil {
		t.Fatalf("validateParticleRestrictionWithKindChange error = %v", err)
	}

	any := &types.AnyElement{
		Namespace:  types.NSCAny,
		MinOccurs:  1,
		MaxOccurs:  1,
		ProcessContents: types.Strict,
	}
	if err := validateParticleRestrictionWithKindChange(schema, &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{any},
	}, &types.ModelGroup{
		Kind:      types.Choice,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA},
	}); err != nil {
		t.Fatalf("validateParticleRestrictionWithKindChange wildcard error = %v", err)
	}

	if !isBlockSuperset(types.DerivationSet(types.DerivationExtension|types.DerivationRestriction), types.DerivationSet(types.DerivationExtension)) {
		t.Fatalf("expected block superset to succeed")
	}

	if !isEffectivelyOptional(&types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{
			&types.ElementDecl{Name: types.QName{Local: "opt"}, MinOccurs: 0, MaxOccurs: 1},
		},
	}) {
		t.Fatalf("expected group to be effectively optional")
	}

	if !isEmptiableParticle(&types.ModelGroup{
		Kind:      types.Choice,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{
			&types.ElementDecl{Name: types.QName{Local: "opt"}, MinOccurs: 0, MaxOccurs: 1},
		},
	}) {
		t.Fatalf("expected choice with optional child to be emptiable")
	}
}

func TestDefaultOrFixedValueValidation(t *testing.T) {
	stringType := types.GetBuiltin(types.TypeName("string"))
	if err := validateDefaultOrFixedValue("value", stringType); err != nil {
		t.Fatalf("unexpected default value error: %v", err)
	}

	idType := types.GetBuiltin(types.TypeName("ID"))
	if err := validateDefaultOrFixedValue("id", idType); err == nil {
		t.Fatalf("expected ID default value error")
	}

	derivedID := &types.SimpleType{
		QName:       types.QName{Namespace: "urn:values", Local: "derivedID"},
		Restriction: &types.Restriction{Base: types.QName{Namespace: types.XSDNamespace, Local: "ID"}},
	}
	if err := validateDefaultOrFixedValue("id", derivedID); err == nil {
		t.Fatalf("expected derived ID default value error")
	}

	ct := types.NewComplexType(types.QName{Namespace: "urn:values", Local: "ct"}, "urn:values")
	ct.SetContent(&types.SimpleContent{
		Extension: &types.Extension{Base: types.QName{Namespace: types.XSDNamespace, Local: "string"}},
	})
	if err := validateDefaultOrFixedValue("text", ct); err != nil {
		t.Fatalf("unexpected simpleContent default value error: %v", err)
	}
}

func TestAttributeGroupStructure(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:ag"
	schema.AttributeFormDefault = parser.Qualified

	attr := &types.AttributeDecl{
		Name: types.QName{Local: "a"},
		Type: types.GetBuiltin(types.TypeName("string")),
	}
	agQName := types.QName{Namespace: "urn:ag", Local: "ag"}
	ag := &types.AttributeGroup{
		Name:       agQName,
		Attributes: []*types.AttributeDecl{attr},
	}
	if err := validateAttributeGroupStructure(schema, agQName, ag); err != nil {
		t.Fatalf("validateAttributeGroupStructure error = %v", err)
	}

	badQName := types.QName{Namespace: "urn:ag", Local: "1bad"}
	if err := validateAttributeGroupStructure(schema, badQName, ag); err == nil {
		t.Fatalf("expected invalid attributeGroup name error")
	}
}

func TestGroupStructure(t *testing.T) {
	schema := parser.NewSchema()
	good := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	if err := validateGroupStructure(schema, types.QName{Local: "g"}, good); err != nil {
		t.Fatalf("validateGroupStructure error = %v", err)
	}

	bad := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 0,
		MaxOccurs: 1,
	}
	if err := validateGroupStructure(schema, types.QName{Local: "g"}, bad); err == nil {
		t.Fatalf("expected invalid group occurrence error")
	}
}

func TestSubstitutionAndDerivationHelpers(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:sub"

	headType := types.NewComplexType(types.QName{Namespace: "urn:sub", Local: "headType"}, "urn:sub")
	headType.SetContent(&types.EmptyContent{})
	memberType := types.NewComplexType(types.QName{Namespace: "urn:sub", Local: "memberType"}, "urn:sub")
	memberType.SetContent(&types.EmptyContent{})
	memberType.ResolvedBase = headType
	memberType.DerivationMethod = types.DerivationRestriction

	headQName := types.QName{Namespace: "urn:sub", Local: "head"}
	memberQName := types.QName{Namespace: "urn:sub", Local: "member"}
	headElem := &types.ElementDecl{Name: headQName, Type: headType}
	memberElem := &types.ElementDecl{Name: memberQName, Type: memberType}

	schema.ElementDecls[headQName] = headElem
	schema.ElementDecls[memberQName] = memberElem
	schema.SubstitutionGroups[headQName] = []types.QName{memberQName}

	if !isSubstitutionGroupMember(schema, headQName, memberQName) {
		t.Fatalf("expected member to be in substitution group")
	}
	if !isSubstitutableElement(schema, headQName, memberQName) {
		t.Fatalf("expected substitutable element")
	}

	headElem.Block = types.DerivationSet(types.DerivationSubstitution)
	if isSubstitutableElement(schema, headQName, memberQName) {
		t.Fatalf("expected substitution to be blocked")
	}

	if !isDerivationBlocked(memberType, headType, types.DerivationSet(types.DerivationRestriction)) {
		t.Fatalf("expected derivation to be blocked")
	}
	if method := derivationMethodForType(memberType); method != types.DerivationRestriction {
		t.Fatalf("unexpected derivation method")
	}
	if !isRestrictionDerivedFrom(memberType, headType) {
		t.Fatalf("expected restriction derivation to succeed")
	}
	if !isRestrictionDerivedFromComplex(memberType, headType) {
		t.Fatalf("expected complex restriction derivation to succeed")
	}
}

func TestParticleRestrictionPaths(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:particles"

	stringType := types.GetBuiltin(types.TypeName("string"))
	elemA := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:particles", Local: "a"},
		Type:      stringType,
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	elemB := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:particles", Local: "b"},
		Type:      stringType,
		MinOccurs: 1,
		MaxOccurs: 1,
	}

	baseSeq := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA, elemB},
	}
	restrSeq := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA},
	}
	if err := validateParticleRestriction(schema, baseSeq, restrSeq); err == nil {
		t.Fatalf("expected sequence restriction error")
	}

	baseAll := &types.ModelGroup{
		Kind:      types.AllGroup,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA, elemB},
	}
	restrAll := &types.ModelGroup{
		Kind:      types.AllGroup,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA},
	}
	if err := validateParticleRestriction(schema, baseAll, restrAll); err == nil {
		t.Fatalf("expected all-group restriction error")
	}

	baseChoiceWildcard := &types.ModelGroup{
		Kind:      types.Choice,
		MinOccurs: 1,
		MaxOccurs: 2,
		Particles: []types.Particle{
			&types.AnyElement{
				Namespace:       types.NSCAny,
				ProcessContents: types.Strict,
				MinOccurs:       1,
				MaxOccurs:       1,
				TargetNamespace: "urn:particles",
			},
			elemA,
		},
	}
	restrElem := &types.ElementDecl{
		Name:      elemA.Name,
		Type:      elemA.Type,
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	if err := validateParticlePairRestriction(schema, baseChoiceWildcard, restrElem); err != nil {
		t.Fatalf("validateParticlePairRestriction error = %v", err)
	}

	baseChoiceElement := &types.ModelGroup{
		Kind:      types.Choice,
		MinOccurs: 1,
		MaxOccurs: 2,
		Particles: []types.Particle{elemA, elemB},
	}
	if err := validateParticlePairRestriction(schema, baseChoiceElement, restrElem); err != nil {
		t.Fatalf("validateParticlePairRestriction element error = %v", err)
	}

	baseAny := &types.AnyElement{
		Namespace:       types.NSCAny,
		ProcessContents: types.Strict,
		MinOccurs:       1,
		MaxOccurs:       2,
		TargetNamespace: "urn:particles",
	}
	restrAny := &types.AnyElement{
		Namespace:       types.NSCTargetNamespace,
		ProcessContents: types.Strict,
		MinOccurs:       1,
		MaxOccurs:       1,
		TargetNamespace: "urn:particles",
	}
	if err := validateParticlePairRestriction(schema, baseAny, restrAny); err != nil {
		t.Fatalf("validateParticlePairRestriction wildcard error = %v", err)
	}

	restrMG := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA, elemB},
	}
	baseAny.Namespace = types.NSCList
	baseAny.NamespaceList = []types.NamespaceURI{"urn:particles"}
	if err := validateParticlePairRestriction(schema, baseAny, restrMG); err != nil {
		t.Fatalf("validateParticlePairRestriction wildcard group error = %v", err)
	}
}

func TestSchemaHelpers(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:schema"

	elemA := &types.ElementDecl{Name: types.QName{Local: "a"}, MinOccurs: 1, MaxOccurs: 1}
	elemB := &types.ElementDecl{Name: types.QName{Local: "b"}, MinOccurs: 1, MaxOccurs: 1}

	baseCT := types.NewComplexType(types.QName{Namespace: "urn:schema", Local: "base"}, "urn:schema")
	baseCT.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: 1,
			MaxOccurs: 1,
			Particles: []types.Particle{elemA},
		},
	})

	derivedCT := types.NewComplexType(types.QName{Namespace: "urn:schema", Local: "derived"}, "urn:schema")
	derivedCT.ResolvedBase = baseCT
	derivedCT.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base:     baseCT.QName,
			Particle: &types.ModelGroup{Kind: types.Sequence, MinOccurs: 1, MaxOccurs: 1, Particles: []types.Particle{elemB}},
		},
	})
	schema.TypeDefs[baseCT.QName] = baseCT
	schema.TypeDefs[derivedCT.QName] = derivedCT

	combined := effectiveContentParticle(schema, derivedCT)
	if combined == nil {
		t.Fatalf("expected combined content particle")
	}

	if groupKindName(types.Sequence) == "" || whiteSpaceName(types.WhiteSpaceCollapse) == "" {
		t.Fatalf("expected group kind and whitespace names")
	}

	if getTypeQName(nil) != (types.QName{}) {
		t.Fatalf("expected empty QName for nil type")
	}

	df := &types.DeferredFacet{FacetName: "minInclusive", FacetValue: "1"}
	listType := types.NewSimpleType(types.QName{Namespace: "urn:schema", Local: "list"}, "urn:schema")
	listType.SetVariety(types.ListVariety)
	if err := validateDeferredFacetApplicability(df, listType, listType.QName); err == nil {
		t.Fatalf("expected deferred facet applicability error for list type")
	}
	if facet, err := convertDeferredFacet(df, types.GetBuiltin(types.TypeName("int"))); err != nil || facet == nil {
		t.Fatalf("expected deferred facet conversion")
	}

	if !isNotationType(types.GetBuiltin(types.TypeName("NOTATION"))) {
		t.Fatalf("expected NOTATION type to be recognized")
	}
}

func TestSimpleTypeHelpers(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:simple"

	bt, baseQName := resolveSimpleContentBaseType(schema, types.QName{Namespace: types.XSDNamespace, Local: "string"})
	if bt == nil || baseQName.Local != "string" {
		t.Fatalf("expected builtin base type resolution")
	}

	baseCT := types.NewComplexType(types.QName{Namespace: "urn:simple", Local: "base"}, "urn:simple")
	baseCT.SetContent(&types.SimpleContent{
		Extension: &types.Extension{Base: types.QName{Namespace: types.XSDNamespace, Local: "string"}},
	})
	schema.TypeDefs[baseCT.QName] = baseCT
	if got, _ := resolveSimpleContentBaseType(schema, baseCT.QName); got == nil {
		t.Fatalf("expected simpleContent base resolution")
	}

	restriction := &types.Restriction{
		Base:   types.QName{Namespace: types.XSDNamespace, Local: string(types.TypeNameAnySimpleType)},
		Facets: []any{&types.Length{Value: 1}},
	}
	if err := validateSimpleContentRestrictionFacets(schema, restriction); err == nil {
		t.Fatalf("expected anySimpleType facet error")
	}

	union := &types.UnionType{}
	if err := validateUnionType(schema, union); err == nil {
		t.Fatalf("expected union member type error")
	}
	union = &types.UnionType{MemberTypes: []types.QName{{Namespace: types.XSDNamespace, Local: "dateTimeStamp"}}}
	if err := validateUnionType(schema, union); err == nil {
		t.Fatalf("expected XSD 1.1 union type error")
	}

	complex := types.NewComplexType(types.QName{Namespace: "urn:simple", Local: "ct"}, "urn:simple")
	schema.TypeDefs[complex.QName] = complex
	union = &types.UnionType{MemberTypes: []types.QName{complex.QName}}
	if err := validateUnionType(schema, union); err == nil {
		t.Fatalf("expected complex member type error")
	}

	if !isXSD11Type("timeDuration") || isXSD11Type("string") {
		t.Fatalf("unexpected XSD 1.1 type detection")
	}
}

func TestPublicWrappers(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:wrap"

	stringType := types.GetBuiltin(types.TypeName("string"))
	childType := types.NewComplexType(types.QName{Namespace: "urn:wrap", Local: "childType"}, "urn:wrap")
	childType.SetContent(&types.EmptyContent{})
	childType.SetAttributes([]*types.AttributeDecl{{
		Name: types.QName{Local: "id"},
		Type: stringType,
	}})
	childElem := &types.ElementDecl{
		Name:      types.QName{Local: "child"},
		Type:      childType,
		MinOccurs: 1,
		MaxOccurs: 1,
	}

	rootType := types.NewComplexType(types.QName{Namespace: "urn:wrap", Local: "rootType"}, "urn:wrap")
	rootType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: 1,
			MaxOccurs: 1,
			Particles: []types.Particle{childElem},
		},
	})

	root := &types.ElementDecl{Name: types.QName{Local: "root"}, Type: rootType}

	if ResolveTypeReference(schema, stringType, false) != stringType {
		t.Fatalf("expected ResolveTypeReference to return same type")
	}
	if !ElementTypesCompatible(stringType, stringType) {
		t.Fatalf("expected ElementTypesCompatible to return true")
	}
	if ResolveSimpleTypeReference(schema, types.QName{Namespace: types.XSDNamespace, Local: "string"}) == nil {
		t.Fatalf("expected ResolveSimpleTypeReference to resolve builtin")
	}
	if !IsIDOnlyType(types.QName{Namespace: types.XSDNamespace, Local: "ID"}) {
		t.Fatalf("expected IsIDOnlyType to return true")
	}
	if IsIDOnlyDerivedType(&types.SimpleType{}) {
		t.Fatalf("expected IsIDOnlyDerivedType to return false")
	}

	field := &types.Field{XPath: "@id"}
	if _, err := ResolveFieldType(schema, field, root, "child"); err != nil {
		t.Fatalf("ResolveFieldType error = %v", err)
	}
	if _, err := ResolveSelectorElementType(schema, root, "child"); err != nil {
		t.Fatalf("ResolveSelectorElementType error = %v", err)
	}
	if len(CollectAllElementDeclarationsFromType(schema, rootType)) == 0 {
		t.Fatalf("expected element collection from type")
	}
}

func TestTraversalHelpers(t *testing.T) {
	elem := &types.ElementDecl{Name: types.QName{Local: "a"}, MinOccurs: 1, MaxOccurs: 1}
	wild := &types.AnyElement{Namespace: types.NSCAny, MinOccurs: 1, MaxOccurs: 1}
	group := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elem, wild},
	}
	content := &types.ElementContent{Particle: group}

	if GetContentParticle(content) != group {
		t.Fatalf("expected content particle to be returned")
	}

	count := 0
	if err := WalkContentParticles(content, func(types.Particle) error {
		count++
		return nil
	}); err != nil {
		t.Fatalf("WalkContentParticles error = %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one content particle, got %d", count)
	}

	seen := 0
	if err := WalkParticles(group, func(types.Particle) error {
		seen++
		return nil
	}); err != nil {
		t.Fatalf("WalkParticles error = %v", err)
	}
	if seen < 3 {
		t.Fatalf("expected to visit group and children")
	}

	if len(CollectElements(group)) != 1 {
		t.Fatalf("expected one element")
	}
	if len(CollectWildcards(group)) != 1 {
		t.Fatalf("expected one wildcard")
	}

	cc := &types.ComplexContent{Extension: &types.Extension{Particle: group}}
	if GetContentParticle(cc) != group {
		t.Fatalf("expected complex content particle to be returned")
	}
}

func TestListTypeValidation(t *testing.T) {
	schema := parser.NewSchema()

	if err := validateListType(schema, &types.ListType{}); err == nil {
		t.Fatalf("expected missing list item type error")
	}

	inline := &types.SimpleType{}
	inline.SetVariety(types.ListVariety)
	inline.List = &types.ListType{ItemType: types.QName{Namespace: types.XSDNamespace, Local: "string"}}
	if err := validateListType(schema, &types.ListType{InlineItemType: inline}); err == nil {
		t.Fatalf("expected list itemType variety error")
	}

	if err := validateListType(schema, &types.ListType{ItemType: types.QName{Namespace: types.XSDNamespace, Local: "string"}}); err != nil {
		t.Fatalf("unexpected list type error: %v", err)
	}

	complex := types.NewComplexType(types.QName{Namespace: "urn:list", Local: "ct"}, "urn:list")
	schema.TypeDefs[complex.QName] = complex
	if err := validateListType(schema, &types.ListType{ItemType: complex.QName}); err == nil {
		t.Fatalf("expected complex list item type error")
	}
}

func TestNotationEnumeration(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:note"
	schema.NamespaceDecls["p"] = "urn:note"

	noteQName := types.QName{Namespace: "urn:note", Local: "note"}
	schema.NotationDecls[noteQName] = &types.NotationDecl{Name: noteQName}

	good := &types.Enumeration{Values: []string{"p:note"}}
	if err := validateNotationEnumeration(schema, []types.Facet{good}, schema.TargetNamespace); err != nil {
		t.Fatalf("validateNotationEnumeration error = %v", err)
	}

	empty := &types.Enumeration{Values: []string{""}}
	if err := validateNotationEnumeration(schema, []types.Facet{empty}, schema.TargetNamespace); err == nil {
		t.Fatalf("expected empty notation enumeration error")
	}

	badPrefix := &types.Enumeration{Values: []string{"x:note"}}
	if err := validateNotationEnumeration(schema, []types.Facet{badPrefix}, schema.TargetNamespace); err == nil {
		t.Fatalf("expected undeclared prefix error")
	}
}

func TestUPAChoiceOverlap(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:upa"

	elem1 := &types.ElementDecl{Name: types.QName{Namespace: "urn:upa", Local: "a"}, MinOccurs: 1, MaxOccurs: 1}
	elem2 := &types.ElementDecl{Name: types.QName{Namespace: "urn:upa", Local: "a"}, MinOccurs: 1, MaxOccurs: 1}
	choice := &types.ModelGroup{
		Kind:      types.Choice,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elem1, elem2},
	}
	if err := validateUPA(schema, &types.ElementContent{Particle: choice}, schema.TargetNamespace); err == nil {
		t.Fatalf("expected choice UPA violation")
	}
}

func TestUPASequenceOverlap(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:upa"

	elem1 := &types.ElementDecl{Name: types.QName{Namespace: "urn:upa", Local: "a"}, MinOccurs: 1, MaxOccurs: 2}
	elem2 := &types.ElementDecl{Name: types.QName{Namespace: "urn:upa", Local: "a"}, MinOccurs: 1, MaxOccurs: 1}
	seq := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elem1, elem2},
	}
	if err := validateUPA(schema, &types.ElementContent{Particle: seq}, schema.TargetNamespace); err == nil {
		t.Fatalf("expected sequence UPA violation")
	}
}

func TestUPAWildcardOverlap(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:upa"

	wild := &types.AnyElement{
		Namespace:       types.NSCAny,
		MinOccurs:       1,
		MaxOccurs:       1,
		TargetNamespace: "urn:upa",
	}
	elem := &types.ElementDecl{Name: types.QName{Namespace: "urn:upa", Local: "a"}, MinOccurs: 1, MaxOccurs: 1}
	choice := &types.ModelGroup{
		Kind:      types.Choice,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{wild, elem},
	}
	if err := validateUPA(schema, &types.ElementContent{Particle: choice}, schema.TargetNamespace); err == nil {
		t.Fatalf("expected wildcard UPA violation")
	}
}

func TestUPAExtensionOverlap(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:upa"

	baseElem := &types.ElementDecl{Name: types.QName{Namespace: "urn:upa", Local: "a"}, MinOccurs: 1, MaxOccurs: 2}
	baseCT := types.NewComplexType(types.QName{Namespace: "urn:upa", Local: "base"}, "urn:upa")
	baseCT.SetContent(&types.ElementContent{Particle: baseElem})
	schema.TypeDefs[baseCT.QName] = baseCT

	extElem := &types.ElementDecl{Name: baseElem.Name, MinOccurs: 1, MaxOccurs: 1}
	extGroup := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{extElem},
	}
	derivedCT := types.NewComplexType(types.QName{Namespace: "urn:upa", Local: "derived"}, "urn:upa")
	derivedCT.DerivationMethod = types.DerivationExtension
	derivedCT.SetContent(&types.ComplexContent{Extension: &types.Extension{Base: baseCT.QName, Particle: extGroup}})

	if err := validateUPA(schema, derivedCT.Content(), schema.TargetNamespace); err == nil {
		t.Fatalf("expected extension UPA violation")
	}
}

func TestWildcardDerivation(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:wild"

	baseAny := &types.AnyElement{
		Namespace:       types.NSCAny,
		ProcessContents: types.Strict,
		MinOccurs:       1,
		MaxOccurs:       1,
		TargetNamespace: "urn:wild",
	}
	baseCT := types.NewComplexType(types.QName{Namespace: "urn:wild", Local: "base"}, "urn:wild")
	baseCT.SetContent(&types.ElementContent{Particle: baseAny})
	schema.TypeDefs[baseCT.QName] = baseCT

	derivedAny := &types.AnyElement{
		Namespace:       types.NSCTargetNamespace,
		ProcessContents: types.Strict,
		MinOccurs:       1,
		MaxOccurs:       1,
		TargetNamespace: "urn:wild",
	}
	derivedCT := types.NewComplexType(types.QName{Namespace: "urn:wild", Local: "derived"}, "urn:wild")
	derivedCT.DerivationMethod = types.DerivationRestriction
	derivedCT.SetContent(&types.ComplexContent{Restriction: &types.Restriction{Base: baseCT.QName, Particle: derivedAny}})

	if err := validateWildcardDerivation(schema, derivedCT); err != nil {
		t.Fatalf("validateWildcardDerivation error = %v", err)
	}
	if !wildcardIsSubset(derivedAny, baseAny) {
		t.Fatalf("expected wildcard subset")
	}
	if processContentsName(types.Strict) != "strict" {
		t.Fatalf("unexpected processContents name")
	}
}

func TestAnyAttributeDerivation(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:anyattr"

	baseAnyAttr := &types.AnyAttribute{
		Namespace:       types.NSCAny,
		ProcessContents: types.Strict,
		TargetNamespace: "urn:anyattr",
	}
	baseCT := types.NewComplexType(types.QName{Namespace: "urn:anyattr", Local: "base"}, "urn:anyattr")
	baseCT.SetContent(&types.EmptyContent{})
	baseCT.SetAnyAttribute(baseAnyAttr)
	schema.TypeDefs[baseCT.QName] = baseCT

	derivedAnyAttr := &types.AnyAttribute{
		Namespace:       types.NSCTargetNamespace,
		ProcessContents: types.Strict,
		TargetNamespace: "urn:anyattr",
	}
	derivedCT := types.NewComplexType(types.QName{Namespace: "urn:anyattr", Local: "derived"}, "urn:anyattr")
	derivedCT.DerivationMethod = types.DerivationRestriction
	derivedCT.SetContent(&types.ComplexContent{Restriction: &types.Restriction{Base: baseCT.QName, AnyAttribute: derivedAnyAttr}})

	if err := validateAnyAttributeDerivation(schema, derivedCT); err != nil {
		t.Fatalf("validateAnyAttributeDerivation error = %v", err)
	}
	if !anyAttributeIsSubset(derivedAnyAttr, baseAnyAttr) {
		t.Fatalf("expected anyAttribute subset")
	}
	if anyAttributeIsSubset(baseAnyAttr, derivedAnyAttr) {
		t.Fatalf("expected anyAttribute superset to fail")
	}
}

func TestUPAHelperFunctions(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:upa"

	elemRepeat := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:upa", Local: "a"},
		MinOccurs: 0,
		MaxOccurs: 2,
	}
	elemSingle := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:upa", Local: "a"},
		MinOccurs: 1,
		MaxOccurs: 1,
	}

	seq1 := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemRepeat},
	}
	seq2 := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemSingle},
	}

	if err := checkModelGroupUPAWithVisited(schema, seq1, seq2, schema.TargetNamespace, make(map[*types.ModelGroup]bool)); err == nil {
		t.Fatalf("expected model group UPA violation")
	}

	if len(collectPossibleLastLeafParticles(seq1, make(map[*types.ModelGroup]bool))) == 0 {
		t.Fatalf("expected last leaf particles")
	}
	if len(collectPossibleFirstLeafParticles(seq2, make(map[*types.ModelGroup]bool))) == 0 {
		t.Fatalf("expected first leaf particles")
	}

	wild1 := &types.AnyElement{Namespace: types.NSCAny, TargetNamespace: "urn:upa"}
	wild2 := &types.AnyElement{Namespace: types.NSCTargetNamespace, TargetNamespace: "urn:upa"}
	if !wildcardsOverlap(wild1, wild2) {
		t.Fatalf("expected wildcards to overlap")
	}
	if !wildcardOverlapsElement(wild1, elemSingle) {
		t.Fatalf("expected wildcard to overlap element")
	}
}

func TestWildcardHelpers(t *testing.T) {
	base := &types.AnyElement{
		Namespace:       types.NSCAny,
		ProcessContents: types.Strict,
		TargetNamespace: "urn:wild",
	}
	derived := &types.AnyElement{
		Namespace:       types.NSCTargetNamespace,
		ProcessContents: types.Strict,
		TargetNamespace: "urn:wild",
	}
	if !wildcardNamespaceSubset(derived, base) {
		t.Fatalf("expected wildcard namespace subset")
	}
	list := &types.AnyElement{
		Namespace:       types.NSCList,
		NamespaceList:   []types.NamespaceURI{"urn:wild"},
		ProcessContents: types.Strict,
		TargetNamespace: "urn:wild",
	}
	if !wildcardNamespaceSubset(list, base) {
		t.Fatalf("expected wildcard list subset")
	}

	anyBase := &types.AnyAttribute{
		Namespace:       types.NSCAny,
		ProcessContents: types.Strict,
		TargetNamespace: "urn:wild",
	}
	anyDerived := &types.AnyAttribute{
		Namespace:       types.NSCTargetNamespace,
		ProcessContents: types.Strict,
		TargetNamespace: "urn:wild",
	}
	if !anyAttributeIsSubset(anyDerived, anyBase) {
		t.Fatalf("expected anyAttribute subset")
	}
}

func TestCompareDateTimeValues(t *testing.T) {
	if cmp, err := compareDateTimeValues("2001-10-26", "2001-10-27", "date"); err != nil || cmp >= 0 {
		t.Fatalf("expected date comparison to order values")
	}
	if _, err := compareDateTimeValues("2001-10-26T21:32:52", "2001-10-26T21:32:52Z", "dateTime"); err == nil {
		t.Fatalf("expected dateTime comparison to be not comparable")
	}
	if cmp, err := compareDateTimeValues("13:20:00Z", "14:20:00Z", "time"); err != nil || cmp >= 0 {
		t.Fatalf("expected time comparison to order values")
	}
}

func TestComplexTypeConstraints(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:ct"

	stringType := types.GetBuiltin(types.TypeName("string"))
	intType := types.GetBuiltin(types.TypeName("int"))

	baseElem := &types.ElementDecl{Name: types.QName{Namespace: "urn:ct", Local: "e"}, Type: stringType}
	baseCT := types.NewComplexType(types.QName{Namespace: "urn:ct", Local: "base"}, "urn:ct")
	baseCT.Final = types.DerivationSet(types.DerivationExtension)
	baseCT.SetContent(&types.ElementContent{Particle: baseElem})
	schema.TypeDefs[baseCT.QName] = baseCT

	extElem := &types.ElementDecl{Name: baseElem.Name, Type: intType}
	derivedCT := types.NewComplexType(types.QName{Namespace: "urn:ct", Local: "derived"}, "urn:ct")
	derivedCT.DerivationMethod = types.DerivationExtension
	derivedCT.SetContent(&types.ComplexContent{Extension: &types.Extension{Base: baseCT.QName, Particle: extElem}})

	if err := validateDerivationConstraints(schema, derivedCT); err == nil {
		t.Fatalf("expected derivation constraint error")
	}
	if err := validateElementDeclarationsConsistent(schema, derivedCT); err == nil {
		t.Fatalf("expected element declarations consistent error")
	}

	idType := types.GetBuiltin(types.TypeName("ID"))
	idCT := types.NewComplexType(types.QName{Namespace: "urn:ct", Local: "idType"}, "urn:ct")
	idCT.SetContent(&types.EmptyContent{})
	idCT.SetAttributes([]*types.AttributeDecl{
		{Name: types.QName{Local: "id1"}, Type: idType},
		{Name: types.QName{Local: "id2"}, Type: idType},
	})
	if err := validateIDAttributeCount(schema, idCT); err == nil {
		t.Fatalf("expected multiple ID attribute error")
	}

	baseSimple := types.NewComplexType(types.QName{Namespace: "urn:ct", Local: "simpleBase"}, "urn:ct")
	baseSimple.SetContent(&types.SimpleContent{Extension: &types.Extension{Base: types.QName{Namespace: types.XSDNamespace, Local: "string"}}})
	schema.TypeDefs[baseSimple.QName] = baseSimple

	cc := &types.ComplexContent{Extension: &types.Extension{Base: baseSimple.QName, Particle: extElem}}
	if err := validateComplexContentStructure(schema, cc); err == nil {
		t.Fatalf("expected complexContent simpleContent extension error")
	}
}

func TestParticleStructureValidation(t *testing.T) {
	schema := parser.NewSchema()
	stringType := types.GetBuiltin(types.TypeName("string"))
	intType := types.GetBuiltin(types.TypeName("int"))

	elemA := &types.ElementDecl{Name: types.QName{Local: "a"}, Type: stringType, MinOccurs: 1, MaxOccurs: 1}
	elemB := &types.ElementDecl{Name: types.QName{Local: "a"}, Type: intType, MinOccurs: 1, MaxOccurs: 1}

	allGroup := &types.ModelGroup{
		Kind:      types.AllGroup,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{elemA, elemB},
	}
	if err := validateParticleStructure(schema, allGroup, nil); err == nil {
		t.Fatalf("expected all-group duplicate element error")
	}

	parentKind := types.Sequence
	if err := validateParticleStructureWithVisited(schema, allGroup, &parentKind, make(map[*types.ModelGroup]bool)); err == nil {
		t.Fatalf("expected nested all-group error")
	}
}

func TestSimpleContentRestrictionFacets(t *testing.T) {
	schema := parser.NewSchema()
	restriction := &types.Restriction{
		Base:   types.QName{Namespace: types.XSDNamespace, Local: "string"},
		Facets: []any{&types.Pattern{Value: "[a-z]+"}},
	}
	if err := validateSimpleContentRestrictionFacets(schema, restriction); err != nil {
		t.Fatalf("unexpected simpleContent facet error: %v", err)
	}
}

func TestResolveTypeReferencePlaceholder(t *testing.T) {
	schema := parser.NewSchema()
	qname := types.QName{Namespace: "urn:types", Local: "t"}
	resolved := types.NewSimpleType(qname, "urn:types")
	resolved.Restriction = &types.Restriction{Base: types.QName{Namespace: types.XSDNamespace, Local: "string"}}
	schema.TypeDefs[qname] = resolved

	placeholder := &types.SimpleType{QName: qname}
	if got := ResolveTypeReference(schema, placeholder, false); got == nil || got == placeholder {
		t.Fatalf("expected placeholder to resolve to schema type")
	}
}

func TestWhiteSpaceRestriction(t *testing.T) {
	derived := &types.SimpleType{}
	derived.SetWhiteSpaceExplicit(types.WhiteSpacePreserve)
	base := types.GetBuiltin(types.TypeName("token"))
	if err := validateWhiteSpaceRestriction(derived, base, base.Name()); err == nil {
		t.Fatalf("expected whiteSpace restriction error")
	}
}

func TestFacetComparisons(t *testing.T) {
	bt := types.GetBuiltin(types.TypeName("decimal"))
	if cmp, err := compareFacetValues("1", "2", bt); err != nil || cmp >= 0 {
		t.Fatalf("expected numeric facet comparison")
	}

	intType := types.GetBuiltin(types.TypeName("integer"))
	if cmp := compareNumericOrString("1", "2", "integer", intType); cmp >= 0 {
		t.Fatalf("expected numeric comparison order")
	}

	minEx := "P2D"
	maxEx := "P1D"
	if err := validateDurationRangeFacets(&minEx, &maxEx, nil, nil); err == nil {
		t.Fatalf("expected invalid duration range")
	}

	minInc := "P1D"
	maxInc := "P2D"
	if err := validateDurationRangeFacets(nil, nil, &minInc, &maxInc); err != nil {
		t.Fatalf("unexpected duration range error: %v", err)
	}
}

func TestCollectAnyAttributeFromGroups(t *testing.T) {
	schema := parser.NewSchema()
	agQName := types.QName{Namespace: "urn:attr", Local: "ag"}
	schema.AttributeGroups[agQName] = &types.AttributeGroup{
		Name:         agQName,
		AnyAttribute: &types.AnyAttribute{Namespace: types.NSCAny},
	}

	attrs := collectAnyAttributeFromGroups(schema, []types.QName{agQName}, nil)
	if len(attrs) != 1 {
		t.Fatalf("expected anyAttribute from groups")
	}
}

func TestResolveFieldTypeSelf(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:field"

	child := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:field", Local: "child"},
		Type:      types.GetBuiltin(types.TypeName("string")),
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	rootType := types.NewComplexType(types.QName{Namespace: "urn:field", Local: "rootType"}, "urn:field")
	rootType.SetContent(&types.ElementContent{Particle: child})
	root := &types.ElementDecl{Name: types.QName{Namespace: "urn:field", Local: "root"}, Type: rootType}

	field := &types.Field{XPath: "."}
	if _, err := resolveFieldType(schema, field, root, "child"); err != nil {
		t.Fatalf("resolveFieldType self error = %v", err)
	}
}

func TestComplexContentStructureVariants(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:cc"

	baseCT := types.NewComplexType(types.QName{Namespace: "urn:cc", Local: "base"}, "urn:cc")
	baseCT.SetContent(&types.EmptyContent{})
	schema.TypeDefs[baseCT.QName] = baseCT

	allGroup := &types.ModelGroup{
		Kind:      types.AllGroup,
		MinOccurs: 1,
		MaxOccurs: 1,
		Particles: []types.Particle{
			&types.ElementDecl{Name: types.QName{Namespace: "urn:cc", Local: "a"}, MinOccurs: 1, MaxOccurs: 1},
		},
	}
	ccExt := &types.ComplexContent{Extension: &types.Extension{Base: baseCT.QName, Particle: allGroup}}
	if err := validateComplexContentStructure(schema, ccExt); err != nil {
		t.Fatalf("unexpected complexContent extension error: %v", err)
	}

	stringType := types.GetBuiltin(types.TypeName("string"))
	baseElem := &types.ElementDecl{Name: types.QName{Namespace: "urn:cc", Local: "b"}, Type: stringType, MinOccurs: 1, MaxOccurs: 1}
	baseContent := &types.ElementContent{Particle: baseElem}
	baseCT2 := types.NewComplexType(types.QName{Namespace: "urn:cc", Local: "base2"}, "urn:cc")
	baseCT2.SetContent(baseContent)
	schema.TypeDefs[baseCT2.QName] = baseCT2

	restrElem := &types.ElementDecl{Name: baseElem.Name, Type: baseElem.Type, MinOccurs: 1, MaxOccurs: 1}
	ccRestr := &types.ComplexContent{Restriction: &types.Restriction{Base: baseCT2.QName, Particle: restrElem}}
	if err := validateComplexContentStructure(schema, ccRestr); err != nil {
		t.Fatalf("unexpected complexContent restriction error: %v", err)
	}
}

func TestSimpleContentStructureValid(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:sc"

	baseCT := types.NewComplexType(types.QName{Namespace: "urn:sc", Local: "base"}, "urn:sc")
	baseCT.SetContent(&types.SimpleContent{Extension: &types.Extension{Base: types.QName{Namespace: types.XSDNamespace, Local: "string"}}})
	baseCT.ResolvedBase = types.GetBuiltin(types.TypeName("string"))
	schema.TypeDefs[baseCT.QName] = baseCT

	restriction := &types.Restriction{
		Base:   baseCT.QName,
		Facets: []any{&types.Length{Value: 1}},
	}
	sc := &types.SimpleContent{Restriction: restriction}
	if err := validateSimpleContentStructure(schema, sc, false); err != nil {
		t.Fatalf("unexpected simpleContent restriction error: %v", err)
	}
}

func TestElementRestrictionValidation(t *testing.T) {
	schema := parser.NewSchema()
	stringType := types.GetBuiltin(types.TypeName("string"))
	intType := types.GetBuiltin(types.TypeName("int"))

	base := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:elem", Local: "e"},
		Type:      stringType,
		MinOccurs: 1,
		MaxOccurs: 1,
		HasFixed:  true,
		Fixed:     "fixed",
	}
	restr := &types.ElementDecl{
		Name:      base.Name,
		Type:      base.Type,
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	if err := validateElementRestriction(schema, base, restr); err == nil {
		t.Fatalf("expected fixed value restriction error")
	}

	restr2 := &types.ElementDecl{
		Name:      base.Name,
		Type:      intType,
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	if err := validateElementRestriction(schema, &types.ElementDecl{Name: base.Name, Type: stringType}, restr2); err == nil {
		t.Fatalf("expected type restriction error")
	}
}
