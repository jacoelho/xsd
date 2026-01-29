package validator

import (
	"bytes"
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

type AttrApplied struct {
	KeyBytes []byte
	Value    runtime.ValueRef
	Name     runtime.SymbolID
	Fixed    bool
	KeyKind  runtime.ValueKind
}

type AttrResult struct {
	Applied []AttrApplied
	Attrs   []StartAttr
}

func (s *Session) ValidateAttributes(typeID runtime.TypeID, attrs []StartAttr, resolver value.NSResolver) (AttrResult, error) {
	if s == nil || s.rt == nil {
		return AttrResult{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	typ, ok := s.typeByID(typeID)
	if !ok {
		return AttrResult{}, fmt.Errorf("type %d not found", typeID)
	}
	storeAttrs := s.hasIdentityConstraints()

	if typ.Kind != runtime.TypeComplex {
		for _, attr := range attrs {
			if !s.isXsiAttribute(&attr) {
				return AttrResult{}, newValidationError(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "attribute not allowed on simple type")
			}
		}
		result := AttrResult{}
		if storeAttrs {
			result.Attrs = make([]StartAttr, 0, len(attrs))
			for _, attr := range attrs {
				attr.Value = s.storeValue(attr.Value)
				attr.KeyKind = runtime.VKInvalid
				attr.KeyBytes = nil
				result.Attrs = append(result.Attrs, attr)
			}
		}
		return result, nil
	}

	ct := s.rt.ComplexTypes[typ.Complex.ID]
	uses := s.attrUses(ct.Attrs)
	present := s.attrPresent
	if cap(present) < len(uses) {
		present = make([]bool, len(uses))
	} else {
		present = present[:len(uses)]
		for i := range present {
			present[i] = false
		}
	}
	s.attrPresent = present
	var validated []StartAttr
	if storeAttrs {
		validated = s.attrValidatedBuf[:0]
		if cap(validated) < len(attrs) {
			validated = make([]StartAttr, 0, len(attrs))
		}
	}
	applied := s.attrAppliedBuf[:0]
	if cap(applied) < len(uses) {
		applied = make([]AttrApplied, 0, len(uses))
	}
	seenID := false

	if err := s.checkDuplicateAttrs(attrs); err != nil {
		return AttrResult{}, err
	}

	for _, attr := range attrs {
		if s.isXsiAttribute(&attr) {
			if storeAttrs {
				attr.Value = s.storeValue(attr.Value)
				attr.KeyKind = runtime.VKInvalid
				attr.KeyBytes = nil
				validated = append(validated, attr)
			}
			continue
		}

		if attr.Sym != 0 {
			if use, idx, ok := lookupAttrUse(s.rt, ct.Attrs, attr.Sym); ok {
				if use.Use == runtime.AttrProhibited {
					return AttrResult{}, newValidationError(xsderrors.ErrAttributeProhibited, "attribute prohibited")
				}
				canon, metrics, err := s.validateValueInternalWithMetrics(use.Validator, attr.Value, resolver, valueOptions{
					applyWhitespace:  true,
					trackIDs:         true,
					requireCanonical: use.Fixed.Present,
					storeValue:       storeAttrs,
				})
				if err != nil {
					return AttrResult{}, wrapValueError(err)
				}
				if s.isIDValidator(use.Validator) {
					if seenID {
						return AttrResult{}, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
					}
					seenID = true
				}
				if storeAttrs {
					attr.Value = canon
					attr.KeyKind = metrics.keyKind
					attr.KeyBytes = metrics.keyBytes
				}
				if use.Fixed.Present {
					fixed := valueBytes(s.rt.Values, use.Fixed)
					if !bytes.Equal(canon, fixed) {
						return AttrResult{}, newValidationError(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
					}
				}
				if idx >= 0 && idx < len(present) {
					present[idx] = true
				}
				if storeAttrs {
					validated = append(validated, attr)
				}
				continue
			}
		}

		if ct.AnyAttr == 0 {
			return AttrResult{}, newValidationError(xsderrors.ErrAttributeNotDeclared, "attribute not declared")
		}
		if !s.rt.WildcardAccepts(ct.AnyAttr, attr.NSBytes, attr.NS) {
			return AttrResult{}, newValidationError(xsderrors.ErrAttributeNotDeclared, "attribute wildcard rejected namespace")
		}

		rule := s.rt.Wildcards[ct.AnyAttr]
		switch rule.PC {
		case runtime.PCSkip:
			if storeAttrs {
				attr.Value = s.storeValue(attr.Value)
				attr.KeyKind = runtime.VKInvalid
				attr.KeyBytes = nil
				validated = append(validated, attr)
			}
			continue
		case runtime.PCLax, runtime.PCStrict:
			if attr.Sym == 0 {
				if rule.PC == runtime.PCStrict {
					return AttrResult{}, newValidationError(xsderrors.ErrValidateWildcardAttrStrictUnresolved, "attribute wildcard strict unresolved")
				}
				if storeAttrs {
					attr.Value = s.storeValue(attr.Value)
					attr.KeyKind = runtime.VKInvalid
					attr.KeyBytes = nil
					validated = append(validated, attr)
				}
				continue
			}
			id, ok := s.globalAttributeBySymbol(attr.Sym)
			if !ok {
				if rule.PC == runtime.PCStrict {
					return AttrResult{}, newValidationError(xsderrors.ErrValidateWildcardAttrStrictUnresolved, "attribute wildcard strict unresolved")
				}
				if storeAttrs {
					attr.Value = s.storeValue(attr.Value)
					attr.KeyKind = runtime.VKInvalid
					attr.KeyBytes = nil
					validated = append(validated, attr)
				}
				continue
			}
			if int(id) >= len(s.rt.Attributes) {
				return AttrResult{}, fmt.Errorf("attribute %d out of range", id)
			}
			globalAttr := s.rt.Attributes[id]
			canon, metrics, err := s.validateValueInternalWithMetrics(globalAttr.Validator, attr.Value, resolver, valueOptions{
				applyWhitespace:  true,
				trackIDs:         true,
				requireCanonical: globalAttr.Fixed.Present,
				storeValue:       storeAttrs,
			})
			if err != nil {
				return AttrResult{}, wrapValueError(err)
			}
			if s.isIDValidator(globalAttr.Validator) {
				if seenID {
					return AttrResult{}, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
				}
				seenID = true
			}
			if storeAttrs {
				attr.Value = canon
				attr.KeyKind = metrics.keyKind
				attr.KeyBytes = metrics.keyBytes
			}
			if globalAttr.Fixed.Present {
				fixed := valueBytes(s.rt.Values, globalAttr.Fixed)
				if !bytes.Equal(canon, fixed) {
					return AttrResult{}, newValidationError(xsderrors.ErrAttributeFixedValue, "fixed attribute value mismatch")
				}
			}
			if storeAttrs {
				validated = append(validated, attr)
			}
		default:
			return AttrResult{}, fmt.Errorf("unknown wildcard processContents %d", rule.PC)
		}
	}

	result := AttrResult{Attrs: validated}
	for i, use := range uses {
		if use.Use == runtime.AttrProhibited {
			continue
		}
		if i < len(present) && present[i] {
			continue
		}
		if use.Use == runtime.AttrRequired {
			return AttrResult{}, newValidationError(xsderrors.ErrRequiredAttributeMissing, "required attribute missing")
		}
		if use.Fixed.Present {
			if s.isIDValidator(use.Validator) {
				if seenID {
					return AttrResult{}, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
				}
				seenID = true
			}
			if err := s.trackDefaultValue(use.Validator, valueBytes(s.rt.Values, use.Fixed)); err != nil {
				return AttrResult{}, err
			}
			if storeAttrs {
				kind, key, err := s.keyForCanonicalValue(use.Validator, valueBytes(s.rt.Values, use.Fixed))
				if err != nil {
					return AttrResult{}, err
				}
				applied = append(applied, AttrApplied{
					Name:     use.Name,
					Value:    use.Fixed,
					Fixed:    true,
					KeyKind:  kind,
					KeyBytes: s.storeKey(key),
				})
			} else {
				applied = append(applied, AttrApplied{Name: use.Name, Value: use.Fixed, Fixed: true})
			}
			continue
		}
		if use.Default.Present {
			if s.isIDValidator(use.Validator) {
				if seenID {
					return AttrResult{}, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
				}
				seenID = true
			}
			if err := s.trackDefaultValue(use.Validator, valueBytes(s.rt.Values, use.Default)); err != nil {
				return AttrResult{}, err
			}
			if storeAttrs {
				kind, key, err := s.keyForCanonicalValue(use.Validator, valueBytes(s.rt.Values, use.Default))
				if err != nil {
					return AttrResult{}, err
				}
				applied = append(applied, AttrApplied{
					Name:     use.Name,
					Value:    use.Default,
					KeyKind:  kind,
					KeyBytes: s.storeKey(key),
				})
			} else {
				applied = append(applied, AttrApplied{Name: use.Name, Value: use.Default})
			}
		}
	}

	result.Applied = applied
	if storeAttrs {
		s.attrValidatedBuf = validated[:0]
	}
	s.attrAppliedBuf = applied[:0]
	return result, nil
}

func (s *Session) attrNamesEqual(a, b *StartAttr) bool {
	if a.Sym != 0 && b.Sym != 0 {
		return a.Sym == b.Sym
	}
	return bytes.Equal(attrNSBytes(s.rt, a), attrNSBytes(s.rt, b)) && bytes.Equal(a.Local, b.Local)
}

func (s *Session) checkDuplicateAttrs(attrs []StartAttr) error {
	if s == nil || len(attrs) < 2 {
		return nil
	}
	// smallAttrDupThreshold switches from quadratic scan to hashing.
	const smallAttrDupThreshold = 8
	if len(attrs) <= smallAttrDupThreshold {
		for i := range attrs {
			for j := i + 1; j < len(attrs); j++ {
				if s.attrNamesEqual(&attrs[i], &attrs[j]) {
					return newValidationError(xsderrors.ErrXMLParse, "duplicate attribute")
				}
			}
		}
		return nil
	}
	size := runtime.NextPow2(len(attrs) * 2)
	table := s.attrSeenTable
	if cap(table) < size {
		table = make([]attrSeenEntry, size)
	} else {
		table = table[:size]
		clear(table)
	}
	mask := uint64(size - 1)

	for i := range attrs {
		nsBytes := attrNSBytes(s.rt, &attrs[i])
		hash := attrNameHash(nsBytes, attrs[i].Local)
		slot := int(hash & mask)
		for {
			entry := table[slot]
			if entry.hash == 0 {
				table[slot] = attrSeenEntry{hash: hash, idx: uint32(i)}
				break
			}
			if entry.hash == hash && s.attrNamesEqual(&attrs[int(entry.idx)], &attrs[i]) {
				s.attrSeenTable = table
				return newValidationError(xsderrors.ErrXMLParse, "duplicate attribute")
			}
			slot = (slot + 1) & int(mask)
		}
	}
	s.attrSeenTable = table
	return nil
}

func attrNSBytes(rt *runtime.Schema, attr *StartAttr) []byte {
	if rt != nil && attr.NS != 0 {
		return rt.Namespaces.Bytes(attr.NS)
	}
	return attr.NSBytes
}

func (s *Session) isXsiAttribute(attr *StartAttr) bool {
	if attr.NS != 0 {
		return attr.NS == s.rt.PredefNS.Xsi
	}
	target := s.rt.Namespaces.Bytes(s.rt.PredefNS.Xsi)
	if len(target) == 0 {
		return false
	}
	return bytes.Equal(target, attr.NSBytes)
}

func (s *Session) isIDValidator(id runtime.ValidatorID) bool {
	if s == nil || s.rt == nil || id == 0 {
		return false
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return false
	}
	meta := s.rt.Validators.Meta[id]
	if meta.Kind != runtime.VString {
		return false
	}
	kind, ok := s.stringKind(meta)
	if !ok {
		return false
	}
	return kind == runtime.StringID
}

func (s *Session) attrUses(ref runtime.AttrIndexRef) []runtime.AttrUse {
	return sliceAttrUses(s.rt.AttrIndex.Uses, ref)
}

func sliceAttrUses(uses []runtime.AttrUse, ref runtime.AttrIndexRef) []runtime.AttrUse {
	if ref.Len == 0 {
		return nil
	}
	off := ref.Off
	end := off + ref.Len
	if int(off) >= len(uses) || int(end) > len(uses) {
		return nil
	}
	return uses[off:end]
}

func lookupAttrUse(rt *runtime.Schema, ref runtime.AttrIndexRef, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
	if rt == nil {
		return runtime.AttrUse{}, -1, false
	}
	uses := sliceAttrUses(rt.AttrIndex.Uses, ref)
	switch ref.Mode {
	case runtime.AttrIndexSortedBinary:
		lo := 0
		hi := len(uses) - 1
		for lo <= hi {
			mid := (lo + hi) / 2
			name := uses[mid].Name
			if name == sym {
				return uses[mid], mid, true
			}
			if name < sym {
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		return runtime.AttrUse{}, -1, false
	case runtime.AttrIndexHash:
		if int(ref.HashTable) >= len(rt.AttrIndex.HashTables) {
			return runtime.AttrUse{}, -1, false
		}
		table := rt.AttrIndex.HashTables[ref.HashTable]
		if len(table.Hash) == 0 || len(table.Slot) == 0 {
			return runtime.AttrUse{}, -1, false
		}
		h := uint64(sym)
		if h == 0 {
			h = 1
		}
		mask := uint64(len(table.Hash) - 1)
		slot := int(h & mask)
		for i := 0; i < len(table.Hash); i++ {
			idx := table.Slot[slot]
			if idx == 0 {
				return runtime.AttrUse{}, -1, false
			}
			if table.Hash[slot] == h {
				useIndex := int(idx - 1)
				if useIndex >= int(ref.Off) && useIndex < int(ref.Off+ref.Len) && useIndex < len(rt.AttrIndex.Uses) {
					use := rt.AttrIndex.Uses[useIndex]
					if use.Name == sym {
						return use, useIndex - int(ref.Off), true
					}
				}
			}
			slot = (slot + 1) & int(mask)
		}
		return runtime.AttrUse{}, -1, false
	default:
		for i, use := range uses {
			if use.Name == sym {
				return use, i, true
			}
		}
		return runtime.AttrUse{}, -1, false
	}
}

func (s *Session) globalAttributeBySymbol(sym runtime.SymbolID) (runtime.AttrID, bool) {
	if sym == 0 {
		return 0, false
	}
	if s.rt == nil || int(sym) >= len(s.rt.GlobalAttributes) {
		return 0, false
	}
	id := s.rt.GlobalAttributes[sym]
	return id, id != 0
}
