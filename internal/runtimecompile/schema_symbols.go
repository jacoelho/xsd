package runtimecompile

import (
	"fmt"
	"slices"

	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/typegraph"
	"github.com/jacoelho/xsd/internal/types"
)

func (b *schemaBuilder) initSymbols() error {
	if b.builder == nil {
		return fmt.Errorf("runtime build: symbol builder missing")
	}
	xsdNS := types.XSDNamespace
	for _, name := range builtinTypeNames() {
		_ = b.internQName(types.QName{Namespace: xsdNS, Local: string(name)})
	}

	for _, entry := range b.registry.TypeOrder {
		if entry.QName.IsZero() {
			continue
		}
		_ = b.internQName(entry.QName)
	}
	for _, entry := range b.registry.ElementOrder {
		_ = b.internQName(entry.QName)
	}
	for _, entry := range b.registry.AttributeOrder {
		_ = b.internQName(entry.QName)
	}
	for _, entry := range b.registry.TypeOrder {
		ct, ok := types.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		attrs, wildcard, err := collectAttributeUses(b.schema, ct)
		if err != nil {
			return err
		}
		for _, attr := range attrs {
			if attr == nil {
				continue
			}
			_ = b.internQName(effectiveAttributeQName(b.schema, attr))
		}
		if wildcard != nil {
			b.internNamespaceConstraint(wildcard.Namespace, wildcard.NamespaceList, wildcard.TargetNamespace)
		}
		particle := typegraph.EffectiveContentParticle(b.schema, ct)
		if particle != nil {
			b.internWildcardNamespaces(particle)
		}
	}
	for _, entry := range b.registry.ElementOrder {
		decl := entry.Decl
		if decl == nil {
			continue
		}
		for _, constraint := range decl.Constraints {
			qname := types.QName{Namespace: constraint.TargetNamespace, Local: constraint.Name}
			_ = b.internQName(qname)
		}
	}
	for _, qname := range schema.SortedQNames(b.schema.NotationDecls) {
		if qname.IsZero() {
			continue
		}
		if id := b.internQName(qname); id != 0 {
			b.notations = append(b.notations, id)
		}
	}
	if len(b.notations) > 1 {
		slices.Sort(b.notations)
		b.notations = slices.Compact(b.notations)
	}
	return nil
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
		types.TypeNameNonPositiveInteger,
		types.TypeNameNegativeInteger,
	}
}
