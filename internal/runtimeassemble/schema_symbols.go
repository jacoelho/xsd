package runtimeassemble

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtimeids"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
	"github.com/jacoelho/xsd/internal/typechain"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/validatorgen"
)

func (b *schemaBuilder) initSymbols() error {
	if b.builder == nil {
		return fmt.Errorf("runtime build: symbol builder missing")
	}
	xsdNS := model.XSDNamespace
	for _, name := range runtimeids.BuiltinTypeNames() {
		b.internQName(model.QName{Namespace: xsdNS, Local: string(name)})
	}

	for _, entry := range b.registry.TypeOrder {
		if entry.QName.IsZero() {
			continue
		}
		b.internQName(entry.QName)
	}
	for _, entry := range b.registry.ElementOrder {
		b.internQName(entry.QName)
	}
	for _, entry := range b.registry.AttributeOrder {
		b.internQName(entry.QName)
	}
	for _, entry := range b.registry.TypeOrder {
		ct, ok := model.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		attrs, wildcard, cached := b.complexTypes.AttributeUses(ct)
		if !cached {
			var err error
			attrs, wildcard, err = validatorgen.CollectAttributeUses(b.schema, ct)
			if err != nil {
				return err
			}
		}
		for _, attr := range attrs {
			if attr == nil {
				continue
			}
			b.internQName(typeresolve.EffectiveAttributeQName(b.schema, attr))
		}
		if wildcard != nil {
			b.internNamespaceConstraint(wildcard.Namespace, wildcard.NamespaceList, wildcard.TargetNamespace)
		}
		particle, ok := b.complexTypes.Content(ct)
		if !ok {
			particle = typechain.EffectiveContentParticle(b.schema, ct)
		}
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
			qname := model.QName{Namespace: constraint.TargetNamespace, Local: constraint.Name}
			b.internQName(qname)
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
