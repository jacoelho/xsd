package validator

import (
	"path"
	"slices"
	"sync"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

// SchemaLocationLoader loads compiled schemas for schemaLocation hints.
type SchemaLocationLoader interface {
	LoadCompiled(location string) (*grammar.CompiledSchema, error)
}

// Option configures a Validator.
type Option func(*Validator)

// WithSchemaLocationLoader sets the loader used for xsi:schemaLocation hints.
func WithSchemaLocationLoader(loader SchemaLocationLoader) Option {
	return func(v *Validator) {
		if v != nil {
			v.schemaLocationLoader = loader
		}
	}
}

// Validator validates XML documents against a CompiledSchema.
type Validator struct {
	grammar                *grammar.CompiledSchema
	baseView               *baseSchemaView
	builtinTypes           map[types.QName]*grammar.CompiledType
	automatonValidatorPool sync.Pool
	schemaLocationLoader   SchemaLocationLoader
}

type validationRun struct {
	validator       *Validator
	schema          schemaView
	ids             map[string]bool
	idrefs          []idrefEntry
	schemaHintCache map[string]*grammar.CompiledSchema
	path            pathStack
	subMatcher      substitutionMatcher
}

// idrefEntry tracks an IDREF value and where it was found during validation.
type idrefEntry struct {
	ref  string
	path string
}

// New creates a new validator for the given compiled schema.
func New(g *grammar.CompiledSchema, opts ...Option) *Validator {
	v := &Validator{
		grammar:      g,
		builtinTypes: make(map[types.QName]*grammar.CompiledType),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(v)
		}
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

func (r *validationRun) mergeSchemaLocationHintsWithRoot(rootPath string, hints []schemaLocationHint) []errors.Validation {
	if len(hints) == 0 || !r.canUseSchemaLocationHints() {
		return nil
	}

	if rootPath == "" {
		rootPath = "/"
	}

	schemaLoader := r.validator.schemaLocationLoader
	if schemaLoader == nil {
		return nil
	}

	var warnings []errors.Validation
	seen := make(map[string]bool)
	seenNamespace := make(map[string]string)
	for _, hint := range hints {
		if prev, ok := seenNamespace[hint.namespace]; ok && prev != hint.location {
			warnings = append(warnings, errors.NewValidationf(errors.ErrSchemaLocationHint, rootPath,
				"conflicting schemaLocation hints for namespace %q: %q vs %q", hint.namespace, prev, hint.location))
			continue
		}
		seenNamespace[hint.namespace] = hint.location

		schemaPath := path.Clean(hint.location)
		key := hint.attribute + ":" + schemaPath
		if seen[key] {
			continue
		}
		seen[key] = true

		var extra *grammar.CompiledSchema
		if cached, ok := r.schemaHintCache[schemaPath]; ok {
			extra = cached
		} else {
			loaded, err := schemaLoader.LoadCompiled(schemaPath)
			if err != nil {
				warnings = append(warnings, errors.NewValidationf(errors.ErrSchemaLocationHint, rootPath,
					"%s hint %q could not be loaded: %v", hint.attribute, hint.location, err))
				continue
			}
			extra = loaded
			r.schemaHintCache[schemaPath] = extra
		}
		if extra == nil {
			continue
		}
		r.mergeSchemaOverlay(extra)
	}

	return warnings
}

func (r *validationRun) canUseSchemaLocationHints() bool {
	if r == nil || r.validator == nil || r.validator.grammar == nil {
		return false
	}
	return r.validator.schemaLocationLoader != nil
}

func (r *validationRun) ensureOverlay() *overlaySchemaView {
	if overlay, ok := r.schema.(*overlaySchemaView); ok {
		return overlay
	}
	overlay := newOverlaySchemaView(r.schema)
	r.schema = overlay
	return overlay
}

func (r *validationRun) mergeSchemaOverlay(extra *grammar.CompiledSchema) {
	if extra == nil {
		return
	}
	overlay := r.ensureOverlay()
	overlay.invalidateConstraintDecls()

	for qname, elem := range extra.Elements {
		if overlay.Element(qname) == nil {
			if overlay.elements == nil {
				overlay.elements = make(map[types.QName]*grammar.CompiledElement)
			}
			overlay.elements[qname] = elem
		}
	}

	for qname, elem := range extra.LocalElements {
		if overlay.LocalElement(qname) == nil {
			if overlay.localElements == nil {
				overlay.localElements = make(map[types.QName]*grammar.CompiledElement)
			}
			overlay.localElements[qname] = elem
		}
	}

	for qname, typ := range extra.Types {
		if overlay.Type(qname) == nil {
			if overlay.types == nil {
				overlay.types = make(map[types.QName]*grammar.CompiledType)
			}
			overlay.types[qname] = typ
		}
	}

	for qname, attr := range extra.Attributes {
		if overlay.Attribute(qname) == nil {
			if overlay.attributes == nil {
				overlay.attributes = make(map[types.QName]*grammar.CompiledAttribute)
			}
			overlay.attributes[qname] = attr
		}
	}

	for qname, not := range extra.NotationDecls {
		if overlay.Notation(qname) == nil {
			if overlay.notationDecls == nil {
				overlay.notationDecls = make(map[types.QName]*types.NotationDecl)
			}
			overlay.notationDecls[qname] = not
		}
	}

	for head, subs := range extra.SubstitutionGroups {
		combined := append([]*grammar.CompiledElement(nil), overlay.SubstitutionGroup(head)...)
		for _, sub := range subs {
			if !containsCompiledElement(combined, sub) {
				combined = append(combined, sub)
			}
		}
		if len(combined) == 0 {
			continue
		}
		if overlay.substitutionGroups == nil {
			overlay.substitutionGroups = make(map[types.QName][]*grammar.CompiledElement)
		}
		overlay.substitutionGroups[head] = combined
	}

	if overlay.elementsWithConstraints == nil {
		overlay.elementsWithConstraints = append([]*grammar.CompiledElement(nil), overlay.base.ElementsWithConstraints()...)
	}
	for _, elem := range extra.ElementsWithConstraints {
		if !containsCompiledElement(overlay.elementsWithConstraints, elem) {
			overlay.elementsWithConstraints = append(overlay.elementsWithConstraints, elem)
		}
	}
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

func containsCompiledElement(list []*grammar.CompiledElement, elem *grammar.CompiledElement) bool {
	return slices.Contains(list, elem)
}
