package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xpath"
)

func (b *schemaBuilder) buildIdentityConstraints() error {
	b.rt.ICSelectors = nil
	b.rt.ICFields = nil
	b.rt.ElemICs = nil

	icByElem := make(map[runtime.ElemID]map[model.QName]runtime.ICID)
	type keyrefPending struct {
		name model.QName
		elem runtime.ElemID
		id   runtime.ICID
	}
	var pending []keyrefPending

	for _, entry := range b.registry.ElementOrder {
		decl := entry.Decl
		if decl == nil || len(decl.Constraints) == 0 {
			continue
		}
		elemID := b.elemIDs[entry.ID]
		elem := b.rt.Elements[elemID]
		off := uint32(len(b.rt.ElemICs))

		for _, constraint := range decl.Constraints {
			icID := runtime.ICID(len(b.rt.ICs))
			selectorOff := uint32(len(b.rt.ICSelectors))
			selectorPrograms, err := xpath.CompilePrograms(constraint.Selector.XPath, constraint.NamespaceContext, xpath.AttributesDisallowed, b.rt)
			if err != nil {
				return fmt.Errorf("runtime build: selector %s: %w", constraint.Name, err)
			}
			for _, program := range selectorPrograms {
				pathID := b.addPath(program)
				b.rt.ICSelectors = append(b.rt.ICSelectors, pathID)
			}
			selectorLen := uint32(len(b.rt.ICSelectors)) - selectorOff

			fieldOff := uint32(len(b.rt.ICFields))
			for fieldIdx, field := range constraint.Fields {
				fieldPrograms, err := xpath.CompilePrograms(field.XPath, constraint.NamespaceContext, xpath.AttributesAllowed, b.rt)
				if err != nil {
					return fmt.Errorf("runtime build: field %d %s: %w", fieldIdx+1, constraint.Name, err)
				}
				for _, program := range fieldPrograms {
					pathID := b.addPath(program)
					b.rt.ICFields = append(b.rt.ICFields, pathID)
				}
				if fieldIdx < len(constraint.Fields)-1 {
					b.rt.ICFields = append(b.rt.ICFields, 0)
				}
			}
			fieldLen := uint32(len(b.rt.ICFields)) - fieldOff

			category := runtime.ICUnique
			switch constraint.Type {
			case model.UniqueConstraint:
				category = runtime.ICUnique
			case model.KeyConstraint:
				category = runtime.ICKey
			case model.KeyRefConstraint:
				category = runtime.ICKeyRef
			}

			name := model.QName{Namespace: constraint.TargetNamespace, Local: constraint.Name}
			nameSym := b.internQName(name)
			ic := runtime.IdentityConstraint{
				Name:        nameSym,
				Category:    category,
				SelectorOff: selectorOff,
				SelectorLen: selectorLen,
				FieldOff:    fieldOff,
				FieldLen:    fieldLen,
			}
			b.rt.ICs = append(b.rt.ICs, ic)
			b.rt.ElemICs = append(b.rt.ElemICs, icID)
			scope := icByElem[elemID]
			if scope == nil {
				scope = make(map[model.QName]runtime.ICID)
				icByElem[elemID] = scope
			}
			scope[name] = icID

			if constraint.Type == model.KeyRefConstraint {
				pending = append(pending, keyrefPending{
					elem: elemID,
					id:   icID,
					name: constraint.ReferQName,
				})
			}
		}

		elem.ICOff = off
		elem.ICLen = uint32(len(b.rt.ElemICs)) - off
		b.rt.Elements[elemID] = elem
	}

	for _, ref := range pending {
		scope := icByElem[ref.elem]
		target, ok := scope[ref.name]
		if !ok {
			return fmt.Errorf("runtime build: keyref %s refers to missing key", ref.name)
		}
		if int(ref.id) >= len(b.rt.ICs) {
			return fmt.Errorf("runtime build: keyref constraint %d out of range", ref.id)
		}
		ic := b.rt.ICs[ref.id]
		ic.Referenced = target
		b.rt.ICs[ref.id] = ic
	}

	return nil
}
