package xsd

import (
	"encoding/xml"
	"strings"
)

func (s *session) validateAttributes(typ typeID, attrs []xml.Attr, line, col int) error {
	rt := s.engine.rt
	if typ.Kind == typeSimple {
		return s.validateSimpleTypeAttributes(attrs, line, col)
	}
	ct := rt.ComplexTypes[typ.ID]
	if ct.Attrs == noAttributeUseSet {
		return nil
	}
	set := rt.AttributeUseSets[ct.Attrs]
	seen := newAttributeSeen(len(set.Uses))
	seenIDAttr := false
	for _, a := range attrs {
		if isNamespaceAttr(a) {
			continue
		}
		if isXSIAttr(a) {
			if err := s.captureIdentityXSIAttribute(a, line, col); err != nil {
				recoverErr := s.recover(err)
				if recoverErr != nil {
					return recoverErr
				}
			}
			continue
		}
		rn := s.runtimeName(a.Name)
		if slot := attributeUseSlot(set, rn); slot >= 0 {
			if err := s.validateDeclaredAttribute(rt, set.Uses[slot], &seen, slot, rn, a.Value, line, col, &seenIDAttr); err != nil {
				recoverErr := s.recover(err)
				if recoverErr != nil {
					return recoverErr
				}
			}
			continue
		}
		handled, err := s.validateWildcardAttribute(rt, set, rn, a.Value, line, col, &seenIDAttr)
		if err != nil {
			recoverErr := s.recover(err)
			if recoverErr != nil {
				return recoverErr
			}
			continue
		}
		if handled {
			continue
		}
		if err := s.recover(validation(ErrValidationAttribute, line, col, s.pathString(), "attribute is not declared: "+rn.Local)); err != nil {
			return err
		}
	}
	if err := s.validateRequiredAndDefaultAttributes(rt, set, seen, line, col, &seenIDAttr); err != nil {
		return err
	}
	return nil
}

type attributeSeen struct {
	list []bool
	mask uint64
}

func newAttributeSeen(n int) attributeSeen {
	if n > 64 {
		return attributeSeen{list: make([]bool, n)}
	}
	return attributeSeen{}
}

func (s *attributeSeen) mark(slot int) bool {
	if s.list != nil {
		if s.list[slot] {
			return false
		}
		s.list[slot] = true
		return true
	}
	bit := uint64(1) << uint(slot)
	if s.mask&bit != 0 {
		return false
	}
	s.mask |= bit
	return true
}

func (s attributeSeen) has(slot int) bool {
	if s.list != nil {
		return s.list[slot]
	}
	return s.mask&(uint64(1)<<uint(slot)) != 0
}

func (s *session) validateSimpleTypeAttributes(attrs []xml.Attr, line, col int) error {
	for _, a := range attrs {
		if isNamespaceAttr(a) {
			continue
		}
		if isXSIAttr(a) {
			if err := s.captureIdentityXSIAttribute(a, line, col); err != nil {
				recoverErr := s.recover(err)
				if recoverErr != nil {
					return recoverErr
				}
			}
			continue
		}
		if err := s.recover(validation(ErrValidationAttribute, line, col, s.pathString(), "simple type does not allow attributes")); err != nil {
			return err
		}
	}
	return nil
}

func attributeUseSlot(set attributeUseSet, rn runtimeName) int {
	if !rn.Known {
		return -1
	}
	for i := range set.Uses {
		if set.Uses[i].Name == rn.Name {
			return i
		}
	}
	return -1
}

func (s *session) validateDeclaredAttribute(rt *runtimeSchema, use attributeUse, seen *attributeSeen, slot int, rn runtimeName, value string, line, col int, seenIDAttr *bool) error {
	if !seen.mark(slot) {
		return validation(ErrValidationAttribute, line, col, s.pathString(), "duplicate attribute "+rn.Local)
	}
	if use.Prohibited {
		return validation(ErrValidationAttribute, line, col, s.pathString(), "prohibited attribute "+rn.Local)
	}
	canon, err := validateSimpleValue(rt, use.Type, value, s.resolveLexicalQNameValue)
	if err != nil {
		if IsUnsupported(err) {
			return err
		}
		return validation(ErrValidationFacet, line, col, s.pathString(), "invalid attribute "+rn.Local+": "+err.Error())
	}
	if err := s.recordAttributeIdentity(use.Type, canon, line, col, seenIDAttr); err != nil {
		return err
	}
	if err := s.captureIdentityAttribute(use.Name, use.Type, canon, line, col); err != nil {
		return err
	}
	return s.validateFixedAttributeValue(rt, use, canon, rn, line, col)
}

func (s *session) validateFixedAttributeValue(rt *runtimeSchema, use attributeUse, canon string, rn runtimeName, line, col int) error {
	if !use.HasFixed {
		return nil
	}
	fixed, err := validateSimpleValue(rt, use.Type, use.Fixed, s.resolveLexicalQNameValue)
	if err != nil {
		if IsUnsupported(err) {
			return err
		}
		return validation(ErrValidationFacet, line, col, s.pathString(), "invalid fixed attribute value")
	}
	if canon != fixed {
		return validation(ErrValidationAttribute, line, col, s.pathString(), "fixed attribute mismatch "+rn.Local)
	}
	return nil
}

func (s *session) validateWildcardAttribute(rt *runtimeSchema, set attributeUseSet, rn runtimeName, value string, line, col int, seenIDAttr *bool) (bool, error) {
	if set.wildcard == noWildcard {
		return false, nil
	}
	w := rt.Wildcards[set.wildcard]
	if !wildcardMatches(rt, w, rn) {
		return false, nil
	}
	if w.Process == processSkip {
		return true, nil
	}
	if rn.Known {
		if id, ok := rt.GlobalAttributes[rn.Name]; ok {
			return true, s.validateKnownWildcardAttribute(rt, rt.Attributes[id], rn, value, line, col, seenIDAttr)
		}
	}
	if w.Process == processLax {
		return true, nil
	}
	if s.hasSchemaLocationHint(rn.NS) {
		return true, s.unsupportedSchemaLocation(line, col, "attribute", rn)
	}
	return false, nil
}

func (s *session) validateKnownWildcardAttribute(rt *runtimeSchema, decl attributeDecl, rn runtimeName, value string, line, col int, seenIDAttr *bool) error {
	canon, err := validateSimpleValue(rt, decl.Type, value, s.resolveLexicalQNameValue)
	if err != nil {
		if IsUnsupported(err) {
			return err
		}
		return validation(ErrValidationFacet, line, col, s.pathString(), "invalid wildcard attribute "+rn.Local)
	}
	if err := s.recordAttributeIdentity(decl.Type, canon, line, col, seenIDAttr); err != nil {
		return err
	}
	return s.captureIdentityAttribute(decl.Name, decl.Type, canon, line, col)
}

func (s *session) validateRequiredAndDefaultAttributes(rt *runtimeSchema, set attributeUseSet, seen attributeSeen, line, col int, seenIDAttr *bool) error {
	for i, use := range set.Uses {
		wasSeen := seen.has(i)
		if use.Required && !wasSeen {
			if err := s.recover(validation(ErrValidationAttribute, line, col, s.pathString(), "missing required attribute")); err != nil {
				return err
			}
			continue
		}
		if !wasSeen && (use.HasDefault || use.HasFixed) {
			value := use.Default
			if use.HasFixed {
				value = use.Fixed
			}
			canon, err := validateSimpleValue(rt, use.Type, value, s.resolveLexicalQNameValue)
			if err != nil {
				if IsUnsupported(err) {
					return err
				}
				if err := s.recover(validation(ErrValidationFacet, line, col, s.pathString(), "invalid default attribute value")); err != nil {
					return err
				}
				continue
			}
			if err := s.recordAttributeIdentity(use.Type, canon, line, col, seenIDAttr); err != nil {
				recoverErr := s.recover(err)
				if recoverErr != nil {
					return recoverErr
				}
				continue
			}
			if err := s.captureIdentityAttribute(use.Name, use.Type, canon, line, col); err != nil {
				recoverErr := s.recover(err)
				if recoverErr != nil {
					return recoverErr
				}
			}
		}
	}
	return nil
}

func (s *session) recordSchemaLocationHints(attrs []xml.Attr) {
	for _, a := range attrs {
		if a.Name.Space != xsiNamespaceURI {
			continue
		}
		switch a.Name.Local {
		case "schemaLocation":
			fields := strings.Fields(a.Value)
			for i := 0; i+1 < len(fields); i += 2 {
				s.addSchemaLocationHint(fields[i])
			}
		case "noNamespaceSchemaLocation":
			if strings.TrimSpace(a.Value) != "" {
				s.addSchemaLocationHint("")
			}
		}
	}
}

func (s *session) addSchemaLocationHint(ns string) {
	if s.schemaLocationNamespaces == nil {
		s.schemaLocationNamespaces = make(map[string]bool)
	}
	s.schemaLocationNamespaces[ns] = true
}

func (s *session) hasSchemaLocationHint(ns string) bool {
	return s.schemaLocationNamespaces != nil && s.schemaLocationNamespaces[ns]
}

func (s *session) unsupportedSchemaLocation(line, col int, component string, rn runtimeName) error {
	return &Error{
		Category: UnsupportedErrorCategory,
		Code:     ErrUnsupportedSchemaHint,
		Line:     line,
		Column:   col,
		Path:     s.pathString(),
		Message:  "xsi:schemaLocation loading is not supported for " + component + " " + rn.Local,
	}
}
