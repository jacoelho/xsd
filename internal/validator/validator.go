package validator

import (
	"slices"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

// Validator validates XML documents against a CompiledSchema.
type Validator struct {
	grammar      *grammar.CompiledSchema
	baseView     *baseSchemaView
	builtinTypes map[types.QName]*grammar.CompiledType
}

type validationRun struct {
	schema     schemaView
	subMatcher substitutionMatcher
	validator  *Validator
	ids        map[string]bool
	idrefs     []idrefEntry
	path       pathStack
}

// idrefEntry tracks an IDREF value and where it was found during schemacheck.
type idrefEntry struct {
	ref    string
	path   string
	line   int
	column int
}

// New creates a new validator for the given compiled schema.
func New(g *grammar.CompiledSchema) *Validator {
	v := &Validator{
		grammar:      g,
		builtinTypes: make(map[types.QName]*grammar.CompiledType),
	}
	v.baseView = newBaseSchemaView(g)
	v.prebuildBuiltinTypes()
	return v
}

// reset clears validation state for a new validation run.
func (r *validationRun) reset() {
	r.ids = make(map[string]bool)
	r.idrefs = nil
	r.path.reset()
}

func (r *validationRun) findElementDeclaration(qname types.QName) *grammar.CompiledElement {
	if decl := r.schema.Element(qname); decl != nil {
		return decl
	}

	if decl := r.findBySubstitution(qname); decl != nil {
		return decl
	}

	return r.schema.LocalElement(qname)
}

func (r *validationRun) findBySubstitution(qname types.QName) *grammar.CompiledElement {
	if head := r.schema.SubstitutionGroupHead(qname); head != nil {
		return head
	}
	return nil
}

func (v *Validator) prebuildBuiltinTypes() {
	for _, name := range builtinTypeNames() {
		bt := types.GetBuiltin(name)
		if bt == nil {
			continue
		}
		buildBuiltinCompiledType(bt, v.builtinTypes)
	}
}

func builtinTypeNames() []types.TypeName {
	return []types.TypeName{
		types.TypeNameAnyType,
		types.TypeNameAnySimpleType,
		types.TypeNameString,
		types.TypeNameBoolean,
		types.TypeNameDecimal,
		types.TypeNameFloat,
		types.TypeNameDouble,
		types.TypeNameDuration,
		types.TypeNameDateTime,
		types.TypeNameTime,
		types.TypeNameDate,
		types.TypeNameGYearMonth,
		types.TypeNameGYear,
		types.TypeNameGMonthDay,
		types.TypeNameGDay,
		types.TypeNameGMonth,
		types.TypeNameHexBinary,
		types.TypeNameBase64Binary,
		types.TypeNameAnyURI,
		types.TypeNameQName,
		types.TypeNameNOTATION,
		types.TypeNameNormalizedString,
		types.TypeNameToken,
		types.TypeNameLanguage,
		types.TypeNameName,
		types.TypeNameNCName,
		types.TypeNameID,
		types.TypeNameIDREF,
		types.TypeNameIDREFS,
		types.TypeNameENTITY,
		types.TypeNameENTITIES,
		types.TypeNameNMTOKEN,
		types.TypeNameNMTOKENS,
		types.TypeNameInteger,
		types.TypeNameLong,
		types.TypeNameInt,
		types.TypeNameShort,
		types.TypeNameByte,
		types.TypeNameNonNegativeInteger,
		types.TypeNamePositiveInteger,
		types.TypeNameUnsignedLong,
		types.TypeNameUnsignedInt,
		types.TypeNameUnsignedShort,
		types.TypeNameUnsignedByte,
		types.TypeNameNegativeInteger,
		types.TypeNameNonPositiveInteger,
	}
}

func (v *Validator) getBuiltinCompiledType(bt *types.BuiltinType) *grammar.CompiledType {
	if bt == nil {
		return nil
	}
	qname := bt.Name()

	if ct := v.builtinTypes[qname]; ct != nil {
		return ct
	}

	return buildBuiltinCompiledType(bt, nil)
}

func buildBuiltinCompiledType(bt *types.BuiltinType, cache map[types.QName]*grammar.CompiledType) *grammar.CompiledType {
	if bt == nil {
		return nil
	}
	qname := bt.Name()
	if cache != nil {
		if ct := cache[qname]; ct != nil {
			return ct
		}
	}

	ct := &grammar.CompiledType{
		QName:    qname,
		Original: bt,
		Kind:     grammar.TypeKindBuiltin,
	}
	ct.IDTypeName = builtinIDTypeName(qname)
	ct.IsNotationType = qname.Namespace == types.XSDNamespace && qname.Local == string(types.TypeNameNOTATION)
	ct.IsQNameOrNotationType = types.IsQNameOrNotation(qname)
	if cache != nil {
		cache[qname] = ct
	}

	base := bt.BaseType()
	if base != nil {
		if baseBuiltin, ok := base.(*types.BuiltinType); ok {
			baseCompiled := buildBuiltinCompiledType(baseBuiltin, cache)
			ct.BaseType = baseCompiled
			ct.DerivationMethod = types.DerivationRestriction
			ct.DerivationChain = append([]*grammar.CompiledType{ct}, baseCompiled.DerivationChain...)
		} else {
			ct.DerivationChain = []*grammar.CompiledType{ct}
		}
	} else {
		ct.DerivationChain = []*grammar.CompiledType{ct}
	}

	return ct
}

func builtinIDTypeName(qname types.QName) string {
	if qname.Namespace != types.XSDNamespace {
		return ""
	}
	switch qname.Local {
	case string(types.TypeNameID), string(types.TypeNameIDREF), string(types.TypeNameIDREFS):
		return qname.Local
	default:
		return ""
	}
}

func containsCompiledElement(list []*grammar.CompiledElement, elem *grammar.CompiledElement) bool {
	return slices.Contains(list, elem)
}
