package valuevalidate

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/facetvalue"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	facetengine "github.com/jacoelho/xsd/internal/schemafacet"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// ErrCircularReference reports a cycle while validating derived types.
var ErrCircularReference = errors.New("circular type reference")

// IDPolicy controls whether ID-only types are accepted.
type IDPolicy uint8

const (
	// IDPolicyAllow allows validating ID-only types.
	IDPolicyAllow IDPolicy = iota
	// IDPolicyDisallow rejects ID-only types for default/fixed values.
	IDPolicyDisallow
)

type mode uint8

const (
	modeDefaultFixed mode = iota
	modeFacet
)

// ValidateDefaultOrFixedResolved validates default/fixed lexical values against resolved types.
func ValidateDefaultOrFixedResolved(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	policy IDPolicy,
) error {
	settings := validationSettings{
		mode:                modeDefaultFixed,
		idPolicy:            policy,
		errorOnPlaceholder:  true,
		requireQNameContext: false,
	}
	return validateValue(schema, value, typ, context, make(map[model.Type]bool), settings)
}

// ValidateWithFacets validates lexical values with schema-time facet conversion.
func ValidateWithFacets(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	convert typeresolve.DeferredFacetConverter,
) error {
	settings := validationSettings{
		mode:                modeFacet,
		idPolicy:            IDPolicyAllow,
		errorOnPlaceholder:  false,
		requireQNameContext: true,
		convert:             convert,
	}
	return validateValue(schema, value, typ, context, make(map[model.Type]bool), settings)
}

type validationSettings struct {
	mode                mode
	idPolicy            IDPolicy
	errorOnPlaceholder  bool
	requireQNameContext bool
	convert             typeresolve.DeferredFacetConverter
}

func validateValue(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	visited map[model.Type]bool,
	settings validationSettings,
) error {
	if typ == nil {
		return nil
	}
	if settings.errorOnPlaceholder {
		if st, ok := typ.(*model.SimpleType); ok && model.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("type %s not resolved", st.QName)
		}
	}
	if visited[typ] {
		return ErrCircularReference
	}
	visited[typ] = true
	defer delete(visited, typ)

	if ct, ok := typ.(*model.ComplexType); ok {
		return validateComplexType(schema, value, ct, context, visited, settings)
	}

	normalized := model.NormalizeWhiteSpace(value, typ)
	if facetvalue.IsQNameOrNotationType(typ) {
		if settings.requireQNameContext && context == nil {
			return fmt.Errorf("namespace context unavailable for QName/NOTATION value")
		}
		if err := facetengine.ValidateQNameContext(normalized, context); err != nil {
			return err
		}
	}

	if typ.IsBuiltin() {
		return validateBuiltin(typ, normalized, settings.idPolicy)
	}

	st, ok := typ.(*model.SimpleType)
	if !ok {
		return nil
	}
	if settings.idPolicy == IDPolicyDisallow && typeresolve.IsIDOnlyDerivedType(schema, st) {
		return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", st.Name().Local)
	}

	switch st.Variety() {
	case model.UnionVariety:
		return validateUnion(schema, normalized, st, context, visited, settings)
	case model.ListVariety:
		return validateList(schema, normalized, st, context, visited, settings)
	default:
		if !facetvalue.IsQNameOrNotationType(st) {
			if err := st.Validate(normalized); err != nil {
				return err
			}
		}
		return facetengine.ValidateSimpleTypeFacets(schema, st, normalized, context, settings.convert)
	}
}

func validateComplexType(
	schema *parser.Schema,
	value string,
	ct *model.ComplexType,
	context map[string]string,
	visited map[model.Type]bool,
	settings validationSettings,
) error {
	sc, ok := ct.Content().(*model.SimpleContent)
	if !ok {
		return nil
	}
	baseType := typeresolve.ResolveSimpleContentBaseTypeFromContent(schema, sc)
	if baseType == nil {
		return nil
	}

	if settings.mode == modeDefaultFixed {
		if sc.Restriction != nil {
			if err := validateValue(schema, value, baseType, context, visited, settings); err != nil {
				return err
			}
			return facetengine.ValidateRestrictionFacets(schema, sc.Restriction, baseType, value, context, settings.convert)
		}
		return validateValue(schema, value, baseType, context, visited, settings)
	}

	if sc.Restriction != nil {
		if err := facetengine.ValidateRestrictionFacets(schema, sc.Restriction, baseType, value, context, settings.convert); err != nil {
			return err
		}
	}
	return validateValue(schema, value, baseType, context, visited, settings)
}

func validateBuiltin(typ model.Type, normalizedValue string, policy IDPolicy) error {
	bt := builtins.GetNS(typ.Name().Namespace, typ.Name().Local)
	if bt == nil {
		return nil
	}
	if policy == IDPolicyDisallow && typeresolve.IsIDOnlyType(typ.Name()) {
		return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
	}
	return bt.Validate(normalizedValue)
}

func validateUnion(
	schema *parser.Schema,
	normalizedValue string,
	st *model.SimpleType,
	context map[string]string,
	visited map[model.Type]bool,
	settings validationSettings,
) error {
	memberTypes := typeresolve.ResolveUnionMemberTypes(schema, st)
	if len(memberTypes) == 0 {
		return fmt.Errorf("union has no member types")
	}

	var (
		firstErr error
		sawCycle bool
	)
	memberSettings := settings
	if settings.mode == modeDefaultFixed {
		memberSettings.idPolicy = IDPolicyAllow
	}
	for _, member := range memberTypes {
		if err := validateValue(schema, normalizedValue, member, context, visited, memberSettings); err == nil {
			return facetengine.ValidateSimpleTypeFacets(schema, st, normalizedValue, context, settings.convert)
		} else if settings.mode == modeDefaultFixed {
			if errors.Is(err, ErrCircularReference) {
				sawCycle = true
			} else if firstErr == nil {
				firstErr = err
			}
		}
	}

	if settings.mode == modeDefaultFixed {
		if firstErr != nil {
			return firstErr
		}
		if sawCycle {
			return fmt.Errorf("cannot validate default/fixed value for circular union type '%s'", st.Name().Local)
		}
		return fmt.Errorf("value '%s' does not match any member type of union '%s'", normalizedValue, st.Name().Local)
	}

	return fmt.Errorf("value %q does not match any member type of union", normalizedValue)
}

func validateList(
	schema *parser.Schema,
	normalizedValue string,
	st *model.SimpleType,
	context map[string]string,
	visited map[model.Type]bool,
	settings validationSettings,
) error {
	itemType := typeresolve.ResolveListItemType(schema, st)
	if itemType == nil {
		return fmt.Errorf("list item type is missing")
	}
	for item := range model.FieldsXMLWhitespaceSeq(normalizedValue) {
		if err := validateValue(schema, item, itemType, context, visited, settings); err != nil {
			if settings.mode == modeDefaultFixed && errors.Is(err, ErrCircularReference) {
				return fmt.Errorf("cannot validate default/fixed value for circular list item type '%s'", st.Name().Local)
			}
			return err
		}
	}
	return facetengine.ValidateSimpleTypeFacets(schema, st, normalizedValue, context, settings.convert)
}
