package valuevalidate

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/facetvalue"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
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
	convert model.DeferredFacetConverter,
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
	convert             model.DeferredFacetConverter
	mode                mode
	idPolicy            IDPolicy
	errorOnPlaceholder  bool
	requireQNameContext bool
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
	if err := validateTypePolicies(schema, typ, settings.errorOnPlaceholder, settings.idPolicy); err != nil {
		return err
	}
	if ct, ok := typ.(*model.ComplexType); ok {
		if visited[typ] {
			return ErrCircularReference
		}
		visited[typ] = true
		defer delete(visited, typ)
		return validateComplexType(schema, value, ct, context, visited, settings)
	}

	if typ.IsBuiltin() {
		normalized := model.NormalizeWhiteSpace(value, typ)
		if facetvalue.IsQNameOrNotationType(typ) {
			if settings.requireQNameContext && context == nil {
				return fmt.Errorf("namespace context unavailable for QName/NOTATION value")
			}
			if err := facets.ValidateQNameContext(normalized, context); err != nil {
				return err
			}
		}
		return validateBuiltin(typ, normalized)
	}

	st, ok := typ.(*model.SimpleType)
	if !ok {
		return nil
	}

	validateTypeWithPolicy := func(current model.Type, scope model.SimpleTypeValidationScope) error {
		idPolicy := settings.idPolicy
		if settings.mode == modeDefaultFixed && scope == model.SimpleTypeValidationScopeUnionMember {
			idPolicy = IDPolicyAllow
		}
		return validateTypePolicies(schema, current, settings.errorOnPlaceholder, idPolicy)
	}
	opts := model.SimpleTypeValidationOptions{
		ResolveListItem: func(current *model.SimpleType) model.Type {
			return typeresolve.ResolveListItemType(schema, current)
		},
		ResolveUnionMembers: func(current *model.SimpleType) []model.Type {
			return typeresolve.ResolveUnionMemberTypes(schema, current)
		},
		ResolveFacetType: func(name model.QName) model.Type {
			return typeresolve.ResolveSimpleTypeReferenceAllowMissing(schema, name)
		},
		ConvertDeferredFacets: settings.convert,
		RequireQNameContext:   settings.requireQNameContext,
		CycleError:            ErrCircularReference,
		ValidateType:          validateTypeWithPolicy,
		ValidateFacets: func(normalized string, current *model.SimpleType, ctx map[string]string) error {
			return facets.ValidateSimpleTypeFacets(schema, current, normalized, ctx, settings.convert)
		},
	}

	switch settings.mode {
	case modeDefaultFixed:
		opts.UnionNoMatch = func(current *model.SimpleType, normalized string, firstErr error, sawCycle bool) error {
			if firstErr != nil {
				return firstErr
			}
			if sawCycle {
				return fmt.Errorf("cannot validate default/fixed value for circular union type '%s'", current.Name().Local)
			}
			return fmt.Errorf("value '%s' does not match any member type of union '%s'", normalized, current.Name().Local)
		}
		opts.ListItemErr = func(current *model.SimpleType, err error, isCycle bool) error {
			if isCycle {
				return fmt.Errorf("cannot validate default/fixed value for circular list item type '%s'", current.Name().Local)
			}
			return err
		}
	default:
		opts.UnionNoMatch = func(_ *model.SimpleType, normalized string, firstErr error, _ bool) error {
			if firstErr != nil {
				return firstErr
			}
			return fmt.Errorf("value %q does not match any member type of union", normalized)
		}
	}

	return model.ValidateSimpleTypeWithOptions(st, value, context, opts)
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
			return facets.ValidateRestrictionFacets(schema, sc.Restriction, baseType, value, context, settings.convert)
		}
		return validateValue(schema, value, baseType, context, visited, settings)
	}

	if sc.Restriction != nil {
		if err := facets.ValidateRestrictionFacets(schema, sc.Restriction, baseType, value, context, settings.convert); err != nil {
			return err
		}
	}
	return validateValue(schema, value, baseType, context, visited, settings)
}

func validateBuiltin(typ model.Type, normalizedValue string) error {
	bt := builtins.GetNS(typ.Name().Namespace, typ.Name().Local)
	if bt == nil {
		return nil
	}
	return bt.Validate(normalizedValue)
}

func validateTypePolicies(schema *parser.Schema, typ model.Type, errorOnPlaceholder bool, policy IDPolicy) error {
	if typ == nil {
		return nil
	}
	if errorOnPlaceholder {
		if st, ok := typ.(*model.SimpleType); ok && model.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("type %s not resolved", st.QName)
		}
	}
	if policy != IDPolicyDisallow {
		return nil
	}
	if typ.IsBuiltin() {
		if typeresolve.IsIDOnlyType(typ.Name()) {
			return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
		}
		return nil
	}
	st, ok := typ.(*model.SimpleType)
	if !ok {
		return nil
	}
	if typeresolve.IsIDOnlyDerivedType(schema, st) {
		return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", st.Name().Local)
	}
	return nil
}
