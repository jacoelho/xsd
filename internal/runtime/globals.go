package runtime

import "errors"

// TypeNameByID resolves a runtime type ID against simple and complex type
// declaration tables.
func TypeNameByID(simpleTypes []SimpleType, complexTypes []ComplexType, typ TypeID) (QName, bool) {
	if !validTypeID(typ, len(simpleTypes), len(complexTypes)) {
		return QName{}, false
	}
	if id, ok := typ.Simple(); ok {
		return simpleTypes[id].Name, true
	}
	id, _ := typ.Complex()
	return complexTypes[id].Name, true
}

// GlobalTypeByName returns a global type ID from a frozen global type read map.
func GlobalTypeByName(reads map[QName]TypeID, derivations TypeDerivationRead, name QName) (TypeID, bool) {
	typ, ok := reads[name]
	if !ok || !validTypeID(typ, derivations.SimpleTypeCount(), derivations.ComplexTypeCount()) {
		return TypeID{}, false
	}
	return typ, true
}

// GlobalAttributeByName returns a global attribute declaration ID from a frozen
// global attribute read map. The final bool distinguishes missing declarations
// from invalid frozen metadata.
func GlobalAttributeByName(reads map[QName]AttributeID, decls []AttributeDeclRead, name QName) (AttributeID, bool, bool) {
	id, ok := reads[name]
	if !ok {
		return 0, false, true
	}
	if !ValidAttributeID(id, len(decls)) {
		return 0, false, false
	}
	return id, true, true
}

// NewNotationReadMap returns the expanded-name read projection for notation
// declarations.
func NewNotationReadMap(names *NameTable, notations map[QName]bool) map[ExpandedName]bool {
	count := notationReadCount(notations)
	if count == 0 || names == nil {
		return nil
	}
	out := make(map[ExpandedName]bool, count)
	for q, present := range notations {
		if !present {
			continue
		}
		out[notationReadName(names, q)] = true
	}
	return out
}

// EqualNotationReadMap reports whether read exposes the same expanded-name
// notation projection as notations.
func EqualNotationReadMap(read map[ExpandedName]bool, names *NameTable, notations map[QName]bool) bool {
	count := notationReadCount(notations)
	if len(read) != count {
		return false
	}
	if count == 0 {
		return true
	}
	if names == nil {
		return false
	}
	for q, present := range notations {
		if !present {
			continue
		}
		got, ok := read[notationReadName(names, q)]
		if !ok || !got {
			return false
		}
	}
	return true
}

// ValidateNotationReadMap validates notation read projections against frozen
// notation declarations.
func ValidateNotationReadMap(read map[ExpandedName]bool, names *NameTable, notations map[QName]bool) error {
	if !EqualNotationReadMap(read, names, notations) {
		return errors.New("notation read map does not match notations")
	}
	return nil
}

func notationReadCount(notations map[QName]bool) int {
	count := 0
	for _, present := range notations {
		if present {
			count++
		}
	}
	return count
}

func notationReadName(names *NameTable, q QName) ExpandedName {
	return ExpandedName{
		Namespace: names.Namespace(q.Namespace),
		Local:     names.Local(q.Local),
	}
}
