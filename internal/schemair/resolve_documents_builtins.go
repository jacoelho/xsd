package schemair

import (
	"cmp"

	ast "github.com/jacoelho/xsd/internal/schemaast"
)

type builtinTypeInfo struct {
	Name       string
	Base       string
	Primitive  string
	Item       string
	Whitespace WhitespaceMode
	Variety    TypeVariety
	Integer    bool
}

func builtinTypes() []builtinTypeInfo {
	return []builtinTypeInfo{
		{Name: "anyType", Primitive: "anySimpleType", Whitespace: WhitespacePreserve},
		{Name: "anySimpleType", Base: "anyType", Primitive: "anySimpleType", Whitespace: WhitespacePreserve},
		{Name: "string", Base: "anySimpleType", Primitive: "string", Whitespace: WhitespacePreserve},
		{Name: "boolean", Base: "anySimpleType", Primitive: "boolean", Whitespace: WhitespaceCollapse},
		{Name: "decimal", Base: "anySimpleType", Primitive: "decimal", Whitespace: WhitespaceCollapse},
		{Name: "float", Base: "anySimpleType", Primitive: "float", Whitespace: WhitespaceCollapse},
		{Name: "double", Base: "anySimpleType", Primitive: "double", Whitespace: WhitespaceCollapse},
		{Name: "duration", Base: "anySimpleType", Primitive: "duration", Whitespace: WhitespaceCollapse},
		{Name: "dateTime", Base: "anySimpleType", Primitive: "dateTime", Whitespace: WhitespaceCollapse},
		{Name: "time", Base: "anySimpleType", Primitive: "time", Whitespace: WhitespaceCollapse},
		{Name: "date", Base: "anySimpleType", Primitive: "date", Whitespace: WhitespaceCollapse},
		{Name: "gYearMonth", Base: "anySimpleType", Primitive: "gYearMonth", Whitespace: WhitespaceCollapse},
		{Name: "gYear", Base: "anySimpleType", Primitive: "gYear", Whitespace: WhitespaceCollapse},
		{Name: "gMonthDay", Base: "anySimpleType", Primitive: "gMonthDay", Whitespace: WhitespaceCollapse},
		{Name: "gDay", Base: "anySimpleType", Primitive: "gDay", Whitespace: WhitespaceCollapse},
		{Name: "gMonth", Base: "anySimpleType", Primitive: "gMonth", Whitespace: WhitespaceCollapse},
		{Name: "hexBinary", Base: "anySimpleType", Primitive: "hexBinary", Whitespace: WhitespaceCollapse},
		{Name: "base64Binary", Base: "anySimpleType", Primitive: "base64Binary", Whitespace: WhitespaceCollapse},
		{Name: "anyURI", Base: "anySimpleType", Primitive: "anyURI", Whitespace: WhitespaceCollapse},
		{Name: "QName", Base: "anySimpleType", Primitive: "QName", Whitespace: WhitespaceCollapse},
		{Name: "NOTATION", Base: "anySimpleType", Primitive: "NOTATION", Whitespace: WhitespaceCollapse},
		{Name: "normalizedString", Base: "string", Primitive: "string", Whitespace: WhitespaceReplace},
		{Name: "token", Base: "normalizedString", Primitive: "string", Whitespace: WhitespaceCollapse},
		{Name: "language", Base: "token", Primitive: "string", Whitespace: WhitespaceCollapse},
		{Name: "Name", Base: "token", Primitive: "string", Whitespace: WhitespaceCollapse},
		{Name: "NCName", Base: "Name", Primitive: "string", Whitespace: WhitespaceCollapse},
		{Name: "ID", Base: "NCName", Primitive: "string", Whitespace: WhitespaceCollapse},
		{Name: "IDREF", Base: "NCName", Primitive: "string", Whitespace: WhitespaceCollapse},
		{Name: "IDREFS", Base: "IDREF", Primitive: "string", Item: "IDREF", Whitespace: WhitespaceCollapse, Variety: TypeVarietyList},
		{Name: "ENTITY", Base: "NCName", Primitive: "string", Whitespace: WhitespaceCollapse},
		{Name: "ENTITIES", Base: "ENTITY", Primitive: "string", Item: "ENTITY", Whitespace: WhitespaceCollapse, Variety: TypeVarietyList},
		{Name: "NMTOKEN", Base: "token", Primitive: "string", Whitespace: WhitespaceCollapse},
		{Name: "NMTOKENS", Base: "NMTOKEN", Primitive: "string", Item: "NMTOKEN", Whitespace: WhitespaceCollapse, Variety: TypeVarietyList},
		{Name: "integer", Base: "decimal", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "long", Base: "integer", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "int", Base: "long", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "short", Base: "int", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "byte", Base: "short", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "nonNegativeInteger", Base: "integer", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "positiveInteger", Base: "nonNegativeInteger", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "unsignedLong", Base: "nonNegativeInteger", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "unsignedInt", Base: "unsignedLong", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "unsignedShort", Base: "unsignedInt", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "unsignedByte", Base: "unsignedShort", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "nonPositiveInteger", Base: "integer", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
		{Name: "negativeInteger", Base: "nonPositiveInteger", Primitive: "decimal", Whitespace: WhitespaceCollapse, Integer: true},
	}
}

func nameFromQName(qname ast.QName) Name {
	return Name{Namespace: qname.Namespace, Local: qname.Local}
}

func docIsZeroName(name Name) bool {
	return name.Namespace == "" && name.Local == ""
}

func derivationSet(set ast.DerivationSet) Derivation {
	var out Derivation
	if set.Has(ast.DerivationExtension) {
		out |= DerivationExtension
	}
	if set.Has(ast.DerivationRestriction) {
		out |= DerivationRestriction
	}
	if set.Has(ast.DerivationList) {
		out |= DerivationList
	}
	if set.Has(ast.DerivationUnion) {
		out |= DerivationUnion
	}
	return out
}

func formatName(name Name) string {
	if name.Namespace == "" {
		return name.Local
	}
	return "{" + name.Namespace + "}" + name.Local
}

func compareName(a, b Name) int {
	if namespace := cmp.Compare(a.Namespace, b.Namespace); namespace != 0 {
		return namespace
	}
	return cmp.Compare(a.Local, b.Local)
}

func facetKind(name string) FacetKind {
	switch name {
	case "pattern":
		return FacetPattern
	case "enumeration":
		return FacetEnumeration
	case "minInclusive":
		return FacetMinInclusive
	case "maxInclusive":
		return FacetMaxInclusive
	case "minExclusive":
		return FacetMinExclusive
	case "maxExclusive":
		return FacetMaxExclusive
	case "minLength":
		return FacetMinLength
	case "maxLength":
		return FacetMaxLength
	case "length":
		return FacetLength
	case "totalDigits":
		return FacetTotalDigits
	case "fractionDigits":
		return FacetFractionDigits
	default:
		return FacetUnknown
	}
}

func whitespaceModeFromString(value string) WhitespaceMode {
	switch value {
	case "replace":
		return WhitespaceReplace
	case "collapse":
		return WhitespaceCollapse
	default:
		return WhitespacePreserve
	}
}

func validWhitespaceRestriction(base, derived WhitespaceMode) bool {
	return whitespaceRestrictiveness(derived) >= whitespaceRestrictiveness(base)
}

func whitespaceRestrictiveness(mode WhitespaceMode) int {
	switch mode {
	case WhitespaceCollapse:
		return 2
	case WhitespaceReplace:
		return 1
	default:
		return 0
	}
}

func whitespaceModeString(mode WhitespaceMode) string {
	switch mode {
	case WhitespaceCollapse:
		return "collapse"
	case WhitespaceReplace:
		return "replace"
	default:
		return "preserve"
	}
}

func fallbackSpecName(spec SimpleTypeSpec) string {
	if spec.BuiltinBase != "" {
		return spec.BuiltinBase
	}
	if spec.Name.Local != "" {
		return spec.Name.Local
	}
	return "string"
}

func groupKindFromAST(kind ast.ParticleKind) GroupKind {
	switch kind {
	case ast.ParticleChoice:
		return GroupChoice
	case ast.ParticleAll:
		return GroupAll
	default:
		return GroupSequence
	}
}
