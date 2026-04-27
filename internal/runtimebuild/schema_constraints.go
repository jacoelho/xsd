package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
	"github.com/jacoelho/xsd/internal/xsdpath"
)

func (b *schemaBuilder) buildIdentityConstraints() error {
	if err := b.assembler.SetIdentitySelectors(nil); err != nil {
		return err
	}
	if err := b.assembler.SetIdentityFields(nil); err != nil {
		return err
	}
	if err := b.assembler.SetElementIdentityConstraints(nil); err != nil {
		return err
	}

	for _, constraint := range b.schema.IdentityConstraints {
		elemID := runtime.ElemID(constraint.Element)
		elem, ok := b.rt.Element(elemID)
		if !ok {
			return fmt.Errorf("runtime build: identity constraint %d element out of range", constraint.ID)
		}
		if elem.ICLen == 0 {
			elem.ICOff = uint32(b.rt.ElementIdentityConstraintCount())
		}

		icID := runtime.ICID(b.rt.IdentityConstraintCount())
		selectorOff := uint32(b.rt.IdentitySelectorCount())
		selectorPrograms, err := compileXPathPrograms(constraint.Selector, constraint.NamespaceContext, xsdpath.AttributesDisallowed, b.rt)
		if err != nil {
			return fmt.Errorf("runtime build: selector %s: %w", formatIRName(constraint.Name), err)
		}
		for _, program := range selectorPrograms {
			pathID := b.addPath(program)
			if _, err := b.assembler.AppendIdentitySelector(pathID); err != nil {
				return err
			}
		}
		selectorLen := uint32(b.rt.IdentitySelectorCount()) - selectorOff

		fieldOff := uint32(b.rt.IdentityFieldCount())
		for fieldIdx, field := range constraint.Fields {
			fieldPrograms, err := compileXPathPrograms(field.XPath, constraint.NamespaceContext, xsdpath.AttributesAllowed, b.rt)
			if err != nil {
				return fmt.Errorf("runtime build: field %d %s: %w", fieldIdx+1, formatIRName(constraint.Name), err)
			}
			for _, program := range fieldPrograms {
				pathID := b.addPath(program)
				if _, err := b.assembler.AppendIdentityField(pathID); err != nil {
					return err
				}
			}
			if fieldIdx < len(constraint.Fields)-1 {
				if _, err := b.assembler.AppendIdentityField(0); err != nil {
					return err
				}
			}
		}
		fieldLen := uint32(b.rt.IdentityFieldCount()) - fieldOff

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
		if _, err := b.assembler.AppendIdentityConstraint(ic); err != nil {
			return err
		}
		if _, err := b.assembler.AppendElementIdentityConstraint(icID); err != nil {
			return err
		}

		elem.ICLen = uint32(b.rt.ElementIdentityConstraintCount()) - elem.ICOff
		if err := b.assembler.SetElement(elemID, elem); err != nil {
			return err
		}
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
