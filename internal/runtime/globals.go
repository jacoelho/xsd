package runtime

import "errors"

// NewElementNameReadsForDecls projects element declaration names for frozen
// runtime publication.
func NewElementNameReadsForDecls(elems []ElementDecl) []QName {
	out := make([]QName, len(elems))
	for i := range elems {
		out[i] = elems[i].Name
	}
	return out
}

// EqualElementNameReadProjectionForDecls reports whether reads expose each
// element declaration's name.
func EqualElementNameReadProjectionForDecls(reads []QName, elems []ElementDecl) bool {
	if len(reads) != len(elems) {
		return false
	}
	for i := range reads {
		if reads[i] != elems[i].Name {
			return false
		}
	}
	return true
}

// ValidateElementNameReadProjectionForDecls validates element-name read
// projections against frozen element declarations.
func ValidateElementNameReadProjectionForDecls(reads []QName, elems []ElementDecl) error {
	if len(reads) != len(elems) {
		return errors.New("element name projection count does not match declarations")
	}
	if !EqualElementNameReadProjectionForDecls(reads, elems) {
		return errors.New("element name projection does not match declaration")
	}
	return nil
}

// TypeNameByID resolves a runtime type ID against simple and complex type
// declaration tables.
func TypeNameByID(simpleTypes []SimpleType, complexTypes []ComplexType, typ TypeID) (QName, bool) {
	if !ValidTypeID(typ, len(simpleTypes), len(complexTypes)) {
		return QName{}, false
	}
	if typ.Kind == TypeSimple {
		return simpleTypes[typ.ID].Name, true
	}
	return complexTypes[typ.ID].Name, true
}

// RootElementByName returns validation start data for a known runtime root
// element name from frozen global and element-start read projections.
func RootElementByName(reads map[QName]ElementID, infos []ElementStartInfo, name RuntimeName) (ElementID, ElementStartInfo, bool) {
	if !name.Known {
		return NoElement, ElementStartInfo{}, false
	}
	id, ok := GlobalElementByName(reads, infos, name.Name)
	if !ok {
		return NoElement, ElementStartInfo{}, false
	}
	info, ok := ElementStartInfoByID(infos, id)
	return id, info, ok
}

// GlobalElementByName returns a global element declaration ID from a frozen
// global element read map.
func GlobalElementByName(reads map[QName]ElementID, infos []ElementStartInfo, name QName) (ElementID, bool) {
	id, ok := reads[name]
	if !ok || !ValidElementID(id, len(infos)) {
		return NoElement, false
	}
	return id, true
}

// GlobalTypeByName returns a global type ID from a frozen global type read map.
func GlobalTypeByName(reads map[QName]TypeID, derivations TypeDerivationRead, name QName) (TypeID, bool) {
	typ, ok := reads[name]
	if !ok || !ValidTypeID(typ, derivations.SimpleTypeCount(), derivations.ComplexTypeCount()) {
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
