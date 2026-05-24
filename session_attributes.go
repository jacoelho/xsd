package xsd

import "encoding/xml"

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
	return s.validateRequiredAndDefaultAttributes(set, seen, line, col, &seenIDAttr)
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
	if slot, ok := set.Index[rn.Name]; ok {
		return int(slot)
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
	identityFields := s.identityAttributeFields(use.Name)
	needs := simpleNeedCanonical
	if len(identityFields) != 0 {
		needs |= simpleNeedIdentity
	}
	simple, err := validateSimpleValueMode(rt, use.Type, value, s.resolveLexicalQNameValue, needs)
	if err != nil {
		if IsUnsupported(err) {
			return err
		}
		return validation(ErrValidationFacet, line, col, s.pathString(), "invalid attribute "+rn.Local+": "+err.Error())
	}
	if err := s.recordAttributeIdentity(simple, line, col, seenIDAttr); err != nil {
		return err
	}
	if err := s.captureIdentityFields(identityFields, simple, line, col); err != nil {
		return err
	}
	return s.validateFixedAttributeValue(use, simple.Canonical, rn, line, col)
}

func (s *session) validateFixedAttributeValue(use attributeUse, canon string, rn runtimeName, line, col int) error {
	if !use.HasFixed {
		return nil
	}
	if canon != use.FixedCanonical {
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
	identityFields := s.identityAttributeFields(decl.Name)
	needs := simpleNeedCanonical
	if len(identityFields) != 0 {
		needs |= simpleNeedIdentity
	}
	simple, err := validateSimpleValueMode(rt, decl.Type, value, s.resolveLexicalQNameValue, needs)
	if err != nil {
		if IsUnsupported(err) {
			return err
		}
		return validation(ErrValidationFacet, line, col, s.pathString(), "invalid wildcard attribute "+rn.Local)
	}
	if err := s.recordAttributeIdentity(simple, line, col, seenIDAttr); err != nil {
		return err
	}
	return s.captureIdentityFields(identityFields, simple, line, col)
}

func (s *session) validateRequiredAndDefaultAttributes(set attributeUseSet, seen attributeSeen, line, col int, seenIDAttr *bool) error {
	for _, slot := range set.Required {
		if !seen.has(int(slot)) {
			if err := s.recover(validation(ErrValidationAttribute, line, col, s.pathString(), "missing required attribute")); err != nil {
				return err
			}
		}
	}
	for _, slot := range set.ValueConstraints {
		if seen.has(int(slot)) {
			continue
		}
		use := set.Uses[slot]
		if use.Required {
			continue
		}
		simple := use.DefaultValue
		if use.HasFixed {
			simple = use.FixedValue
		}
		if err := s.recordAttributeIdentity(simple, line, col, seenIDAttr); err != nil {
			recoverErr := s.recover(err)
			if recoverErr != nil {
				return recoverErr
			}
			continue
		}
		if err := s.captureIdentityFields(s.identityAttributeFields(use.Name), simple, line, col); err != nil {
			recoverErr := s.recover(err)
			if recoverErr != nil {
				return recoverErr
			}
		}
	}
	return nil
}

func (s *session) recordSchemaLocationHints(attrs []xml.Attr, line, col int) error {
	for _, a := range attrs {
		if a.Name.Space != xsiNamespaceURI {
			continue
		}
		switch a.Name.Local {
		case "schemaLocation":
			if err := s.recordNamespaceSchemaLocationHints(a.Value, line, col); err != nil {
				return err
			}
		case "noNamespaceSchemaLocation":
			if err := s.recordNoNamespaceSchemaLocationHint(a.Value, line, col); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *session) recordNamespaceSchemaLocationHints(value string, line, col int) error {
	count := 0
	for field := range xmlFieldsSeq(value) {
		if !isAnyURI(field) {
			return validation(ErrValidationAttribute, line, col, s.pathString(), "invalid xsi:schemaLocation URI "+field)
		}
		count++
	}
	if count%2 != 0 {
		return validation(ErrValidationAttribute, line, col, s.pathString(), "xsi:schemaLocation must contain namespace/location pairs")
	}
	i := 0
	for field := range xmlFieldsSeq(value) {
		if i%2 == 0 {
			s.addSchemaLocationHint(field)
		}
		i++
	}
	return nil
}

func (s *session) recordNoNamespaceSchemaLocationHint(value string, line, col int) error {
	value = trimXMLWhitespace(value)
	if value == "" {
		return validation(ErrValidationAttribute, line, col, s.pathString(), "xsi:noNamespaceSchemaLocation is empty")
	}
	if !isAnyURI(value) {
		return validation(ErrValidationAttribute, line, col, s.pathString(), "invalid xsi:noNamespaceSchemaLocation URI "+value)
	}
	s.addSchemaLocationHint("")
	return nil
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
