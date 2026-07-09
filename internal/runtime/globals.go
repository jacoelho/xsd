package runtime

import (
	"errors"
	"maps"
)

// RuntimeGlobals is the frozen global symbol metadata needed to validate global
// declaration maps.
type RuntimeGlobals struct {
	GlobalAttributes map[QName]AttributeID
	GlobalElements   map[QName]ElementID
	GlobalTypes      map[QName]TypeID
	GlobalIdentities map[QName]IdentityConstraintID
	Notations        map[QName]bool
	AttributeNames   []QName
	ElementNames     []QName
	SimpleTypeNames  []QName
	ComplexTypeNames []QName
	IdentityNames    []QName
}

// RuntimeGlobalInput is the mutable runtime state needed to build frozen global
// declaration validation metadata.
type RuntimeGlobalInput struct {
	GlobalAttributes map[QName]AttributeID
	GlobalElements   map[QName]ElementID
	GlobalTypes      map[QName]TypeID
	GlobalIdentities map[QName]IdentityConstraintID
	Notations        map[QName]bool
	Attributes       []AttributeDecl
	Elements         []ElementDecl
	SimpleTypes      []SimpleType
	ComplexTypes     []ComplexType
	Identities       []IdentityConstraint
}

// GlobalDeclarationMaps are the global symbol maps that validation can read by
// expanded name.
type GlobalDeclarationMaps struct {
	Attributes map[QName]AttributeID
	Elements   map[QName]ElementID
	Types      map[QName]TypeID
}

// GlobalReadMaps are the freeze-published read projections for global
// declaration lookup.
type GlobalReadMaps struct {
	Attributes map[QName]AttributeID
	Elements   map[QName]ElementID
	Types      map[QName]TypeID
}

// NewGlobalDeclarationMaps projects global declaration maps into the runtime
// shape used for read-map publication and invariant validation.
func NewGlobalDeclarationMaps(attributes map[QName]AttributeID, elements map[QName]ElementID, types map[QName]TypeID) GlobalDeclarationMaps {
	return GlobalDeclarationMaps{
		Attributes: attributes,
		Elements:   elements,
		Types:      types,
	}
}

// NewGlobalReadMapProjection projects already-published read maps into the
// runtime shape used for invariant validation.
func NewGlobalReadMapProjection(attributes map[QName]AttributeID, elements map[QName]ElementID, types map[QName]TypeID) GlobalReadMaps {
	return GlobalReadMaps{
		Attributes: attributes,
		Elements:   elements,
		Types:      types,
	}
}

// ElementNameReadShape is the source projection for one element name read.
type ElementNameReadShape struct {
	Name QName
}

// NewElementNameReads projects element declaration names for frozen runtime
// publication.
func NewElementNameReads(shapes []ElementNameReadShape) []QName {
	out := make([]QName, len(shapes))
	for i := range shapes {
		out[i] = shapes[i].Name
	}
	return out
}

// NewElementNameReadsForDecls projects element declaration names for frozen
// runtime publication.
func NewElementNameReadsForDecls(elems []ElementDecl) []QName {
	return NewElementNameReads(elementNameReadShapes(elems))
}

// ElementNameByID resolves an element name ID against an element-name
// projection table.
func ElementNameByID(names []QName, id ElementID) (QName, bool) {
	if !ValidElementID(id, len(names)) {
		return QName{}, false
	}
	return names[id], true
}

// EqualElementNameReadProjection reports whether reads expose each element
// declaration's name.
func EqualElementNameReadProjection(reads []QName, shapes []ElementNameReadShape) bool {
	if len(reads) != len(shapes) {
		return false
	}
	for i := range reads {
		if reads[i] != shapes[i].Name {
			return false
		}
	}
	return true
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

// NewRuntimeGlobals returns cloned global declaration validation metadata.
func NewRuntimeGlobals(in RuntimeGlobalInput) RuntimeGlobals {
	return CloneRuntimeGlobals(RuntimeGlobals{
		GlobalAttributes: in.GlobalAttributes,
		GlobalElements:   in.GlobalElements,
		GlobalTypes:      in.GlobalTypes,
		GlobalIdentities: in.GlobalIdentities,
		Notations:        in.Notations,
		AttributeNames:   AttributeDeclNames(in.Attributes),
		ElementNames:     NewElementNameReadsForDecls(in.Elements),
		SimpleTypeNames:  SimpleTypeNames(in.SimpleTypes),
		ComplexTypeNames: ComplexTypeNames(in.ComplexTypes),
		IdentityNames:    IdentityConstraintNames(in.Identities),
	})
}

// AttributeDeclNames returns declaration names in attribute table order.
func AttributeDeclNames(attrs []AttributeDecl) []QName {
	names := make([]QName, len(attrs))
	for i, attr := range attrs {
		names[i] = attr.Name
	}
	return names
}

// SimpleTypeNames returns type names in simple-type table order.
func SimpleTypeNames(types []SimpleType) []QName {
	names := make([]QName, len(types))
	for i, typ := range types {
		names[i] = typ.Name
	}
	return names
}

// ComplexTypeNames returns type names in complex-type table order.
func ComplexTypeNames(types []ComplexType) []QName {
	names := make([]QName, len(types))
	for i, typ := range types {
		names[i] = typ.Name
	}
	return names
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

// IdentityConstraintNames returns constraint names in identity table order.
func IdentityConstraintNames(identities []IdentityConstraint) []QName {
	names := make([]QName, len(identities))
	for i, identity := range identities {
		names[i] = identity.Name
	}
	return names
}

// ValidateRuntimeGlobals validates frozen global declaration maps.
func ValidateRuntimeGlobals(names *NameTable, globals RuntimeGlobals) error {
	if names == nil {
		return errors.New("runtime globals require name table")
	}
	for q, id := range globals.GlobalAttributes {
		if !names.ValidQName(q) || !validAttributeID(globals, id) {
			return errors.New("global attribute references invalid declaration")
		}
		if globals.AttributeNames[id] != q {
			return errors.New("global attribute name does not match declaration")
		}
	}
	for q, id := range globals.GlobalElements {
		if !names.ValidQName(q) || !validElementID(globals, id) {
			return errors.New("global element references invalid declaration")
		}
		if globals.ElementNames[id] != q {
			return errors.New("global element name does not match declaration")
		}
	}
	for q, typ := range globals.GlobalTypes {
		name, ok := globalTypeName(globals, typ)
		if !names.ValidQName(q) || !ok {
			return errors.New("global type references invalid declaration")
		}
		if name != q {
			return errors.New("global type name does not match declaration")
		}
	}
	for q, id := range globals.GlobalIdentities {
		if !names.ValidQName(q) || !validIdentityID(globals, id) {
			return errors.New("global identity references invalid declaration")
		}
		if globals.IdentityNames[id] != q {
			return errors.New("global identity name does not match declaration")
		}
	}
	for q := range globals.Notations {
		if !names.ValidQName(q) {
			return errors.New("notation references invalid name")
		}
	}
	return nil
}

// NewGlobalReadMaps returns the validation-facing global declaration read maps.
func NewGlobalReadMaps(globals GlobalDeclarationMaps) GlobalReadMaps {
	return GlobalReadMaps{
		Attributes: maps.Clone(globals.Attributes),
		Elements:   maps.Clone(globals.Elements),
		Types:      maps.Clone(globals.Types),
	}
}

// EqualGlobalAttributeReadMap reports whether the attribute read map exposes
// the same declarations as globals.
func EqualGlobalAttributeReadMap(reads GlobalReadMaps, globals GlobalDeclarationMaps) bool {
	return maps.Equal(reads.Attributes, globals.Attributes)
}

// EqualGlobalElementReadMap reports whether the element read map exposes the
// same declarations as globals.
func EqualGlobalElementReadMap(reads GlobalReadMaps, globals GlobalDeclarationMaps) bool {
	return maps.Equal(reads.Elements, globals.Elements)
}

// EqualGlobalTypeReadMap reports whether the type read map exposes the same
// declarations as globals.
func EqualGlobalTypeReadMap(reads GlobalReadMaps, globals GlobalDeclarationMaps) bool {
	return maps.Equal(reads.Types, globals.Types)
}

// ValidateGlobalReadMaps validates global declaration read maps against frozen
// global declaration maps.
func ValidateGlobalReadMaps(reads GlobalReadMaps, globals GlobalDeclarationMaps) error {
	if !EqualGlobalAttributeReadMap(reads, globals) {
		return errors.New("global attribute read map does not match globals")
	}
	if !EqualGlobalElementReadMap(reads, globals) {
		return errors.New("global element read map does not match globals")
	}
	if !EqualGlobalTypeReadMap(reads, globals) {
		return errors.New("global type read map does not match globals")
	}
	return nil
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

func elementNameReadShapes(elems []ElementDecl) []ElementNameReadShape {
	out := make([]ElementNameReadShape, len(elems))
	for i := range elems {
		out[i] = ElementNameReadShape{
			Name: elems[i].Name,
		}
	}
	return out
}

func globalTypeName(globals RuntimeGlobals, typ TypeID) (QName, bool) {
	if !ValidTypeID(typ, len(globals.SimpleTypeNames), len(globals.ComplexTypeNames)) {
		return QName{}, false
	}
	switch typ.Kind {
	case TypeSimple:
		return globals.SimpleTypeNames[typ.ID], true
	case TypeComplex:
		return globals.ComplexTypeNames[typ.ID], true
	default:
		return QName{}, false
	}
}

func validAttributeID(globals RuntimeGlobals, id AttributeID) bool {
	return ValidAttributeID(id, len(globals.AttributeNames))
}

func validElementID(globals RuntimeGlobals, id ElementID) bool {
	return ValidElementID(id, len(globals.ElementNames))
}

func validIdentityID(globals RuntimeGlobals, id IdentityConstraintID) bool {
	return ValidIdentityConstraintID(id, len(globals.IdentityNames))
}
