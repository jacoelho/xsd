package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

func (b *schemaBuilder) buildIdentityConstraints() error {
	b.rt.ICSelectors = nil
	b.rt.ICFields = nil
	b.rt.ElemICs = nil

	for _, constraint := range b.schema.IdentityConstraints {
		elemID := runtime.ElemID(constraint.Element)
		if elemID == 0 || int(elemID) >= len(b.rt.Elements) {
			return fmt.Errorf("runtime build: identity constraint %d element out of range", constraint.ID)
		}
		elem := b.rt.Elements[elemID]
		if elem.ICLen == 0 {
			elem.ICOff = uint32(len(b.rt.ElemICs))
		}

		icID := runtime.ICID(len(b.rt.ICs))
		selectorOff := uint32(len(b.rt.ICSelectors))
		selectorExpr, err := runtime.Parse(constraint.Selector, constraint.NamespaceContext, runtime.AttributesDisallowed)
		if err != nil {
			return fmt.Errorf("runtime build: selector %s: %w", formatIRName(constraint.Name), err)
		}
		selectorPrograms, err := runtime.CompileExpression(selectorExpr, b.rt)
		if err != nil {
			return fmt.Errorf("runtime build: selector %s: %w", formatIRName(constraint.Name), err)
		}
		for _, program := range selectorPrograms {
			pathID := b.addPath(program)
			b.rt.ICSelectors = append(b.rt.ICSelectors, pathID)
		}
		selectorLen := uint32(len(b.rt.ICSelectors)) - selectorOff

		fieldOff := uint32(len(b.rt.ICFields))
		for fieldIdx, field := range constraint.Fields {
			fieldExpr, err := runtime.Parse(field.XPath, constraint.NamespaceContext, runtime.AttributesAllowed)
			if err != nil {
				return fmt.Errorf("runtime build: field %d %s: %w", fieldIdx+1, formatIRName(constraint.Name), err)
			}
			fieldPrograms, err := runtime.CompileExpression(fieldExpr, b.rt)
			if err != nil {
				return fmt.Errorf("runtime build: field %d %s: %w", fieldIdx+1, formatIRName(constraint.Name), err)
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

		nameSym, err := b.lookupIRSymbol(constraint.Name)
		if err != nil {
			return err
		}
		ic := runtime.IdentityConstraint{
			Name:        nameSym,
			Category:    identityCategory(constraint.Kind),
			SelectorOff: selectorOff,
			SelectorLen: selectorLen,
			FieldOff:    fieldOff,
			FieldLen:    fieldLen,
		}
		if constraint.Kind == schemair.IdentityKeyRef {
			ic.Referenced = runtime.ICID(constraint.ReferID)
		}
		b.rt.ICs = append(b.rt.ICs, ic)
		b.rt.ElemICs = append(b.rt.ElemICs, icID)

		elem.ICLen = uint32(len(b.rt.ElemICs)) - elem.ICOff
		b.rt.Elements[elemID] = elem
	}

	return nil
}

func identityCategory(kind schemair.IdentityKind) runtime.ICCategory {
	switch kind {
	case schemair.IdentityKey:
		return runtime.ICKey
	case schemair.IdentityKeyRef:
		return runtime.ICKeyRef
	default:
		return runtime.ICUnique
	}
}
