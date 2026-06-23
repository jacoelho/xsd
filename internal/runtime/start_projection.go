package runtime

import "errors"

// ElementStartInfo is the runtime-published data needed to start validating an
// element declaration.
type ElementStartInfo struct {
	Type     TypeID
	Block    DerivationMask
	Abstract bool
	Nillable bool
	Fixed    bool
	Default  bool
}

// ElementStartInfoShape is the schema-independent projection used to publish
// element start metadata.
type ElementStartInfoShape struct {
	Type     TypeID
	Block    DerivationMask
	Abstract bool
	Nillable bool
	Fixed    bool
	Default  bool
}

// ElementStartDeclShape is the declaration facts needed to publish element
// start metadata.
type ElementStartDeclShape struct {
	Type     TypeID
	Block    DerivationMask
	Abstract bool
	Nillable bool
	Fixed    bool
	Default  bool
}

// NewElementStartInfo returns the start projection for one element
// declaration.
func NewElementStartInfo(shape ElementStartInfoShape) ElementStartInfo {
	return ElementStartInfo(shape)
}

// NewElementStartInfoForDecl returns the start projection for one element
// declaration shape.
func NewElementStartInfoForDecl(shape ElementStartDeclShape) ElementStartInfo {
	return NewElementStartInfo(ElementStartInfoShape(shape))
}

// NewElementStartInfosForDecls returns start projections for element
// declaration shapes.
func NewElementStartInfosForDecls(shapes []ElementStartDeclShape) []ElementStartInfo {
	out := make([]ElementStartInfo, len(shapes))
	for i := range shapes {
		out[i] = NewElementStartInfoForDecl(shapes[i])
	}
	return out
}

// NewElementStartInfoForElementDecl returns the start projection for one frozen
// runtime element declaration.
func NewElementStartInfoForElementDecl(decl ElementDecl) ElementStartInfo {
	return NewElementStartInfo(ElementStartInfoShape{
		Type:     decl.Type,
		Block:    decl.Block,
		Abstract: decl.Abstract,
		Nillable: decl.Nillable,
		Fixed:    decl.Fixed != nil,
		Default:  decl.Default != nil,
	})
}

// NewElementStartInfosForElementDecls returns start projections for frozen
// runtime element declarations.
func NewElementStartInfosForElementDecls(decls []ElementDecl) []ElementStartInfo {
	out := make([]ElementStartInfo, len(decls))
	for i := range decls {
		out[i] = NewElementStartInfoForElementDecl(decls[i])
	}
	return out
}

// DeclaredElementTypeByID returns the declared type for an element from the
// frozen start projection table.
func DeclaredElementTypeByID(infos []ElementStartInfo, id ElementID) (TypeID, bool) {
	if !ValidElementID(id, len(infos)) {
		return TypeID{}, false
	}
	return infos[id].Type, true
}

// ElementStartInfoByID returns validation start data for an element from the
// frozen start projection table.
func ElementStartInfoByID(infos []ElementStartInfo, id ElementID) (ElementStartInfo, bool) {
	if !ValidElementID(id, len(infos)) {
		return ElementStartInfo{}, false
	}
	return infos[id], true
}

// EqualElementStartInfo reports whether two element start projections expose
// the same runtime-facing facts.
func EqualElementStartInfo(a, b ElementStartInfo) bool {
	return a == b
}

// EqualElementStartInfoForDecl reports whether info exposes the start
// projection for shape.
func EqualElementStartInfoForDecl(info ElementStartInfo, shape ElementStartDeclShape) bool {
	return EqualElementStartInfo(info, NewElementStartInfoForDecl(shape))
}

// EqualElementStartInfosForDecls reports whether infos exposes the start
// projections for shapes.
func EqualElementStartInfosForDecls(infos []ElementStartInfo, shapes []ElementStartDeclShape) bool {
	if len(infos) != len(shapes) {
		return false
	}
	for len(infos) > 0 {
		if !EqualElementStartInfoForDecl(infos[0], shapes[0]) {
			return false
		}
		infos = infos[1:]
		shapes = shapes[1:]
	}
	return true
}

// EqualElementStartInfoForElementDecl reports whether info exposes the start
// projection for decl.
func EqualElementStartInfoForElementDecl(info ElementStartInfo, decl ElementDecl) bool {
	return EqualElementStartInfo(info, NewElementStartInfoForElementDecl(decl))
}

// EqualElementStartInfosForElementDecls reports whether infos expose the start
// projections for decls.
func EqualElementStartInfosForElementDecls(infos []ElementStartInfo, decls []ElementDecl) bool {
	if len(infos) != len(decls) {
		return false
	}
	for len(infos) > 0 {
		if !EqualElementStartInfoForElementDecl(infos[0], decls[0]) {
			return false
		}
		infos = infos[1:]
		decls = decls[1:]
	}
	return true
}

// ValidateElementStartInfosForElementDecls validates an element-start
// projection table against runtime element declarations.
func ValidateElementStartInfosForElementDecls(infos []ElementStartInfo, decls []ElementDecl) error {
	if len(infos) != len(decls) {
		return errors.New("element start projection count does not match declarations")
	}
	if !EqualElementStartInfosForElementDecls(infos, decls) {
		return errors.New("element start projection does not match declaration")
	}
	return nil
}

// TypeInfo is the runtime-published data needed to start validating a runtime
// type.
type TypeInfo struct {
	Block    DerivationMask
	Abstract bool
}

// TypeInfoShape is the schema-independent projection used to publish type
// start metadata.
type TypeInfoShape struct {
	Block    DerivationMask
	Abstract bool
}

// NewTypeInfo returns the start projection for one runtime type.
func NewTypeInfo(shape TypeInfoShape) TypeInfo {
	return TypeInfo(shape)
}

// NewTypeInfoForComplexType returns the start projection for one complex type.
func NewTypeInfoForComplexType(ct ComplexType) TypeInfo {
	return NewTypeInfo(TypeInfoShape{
		Block:    ct.Block,
		Abstract: ct.Abstract,
	})
}

// NewTypeInfosForComplexTypes returns start projections for complex types.
func NewTypeInfosForComplexTypes(complexTypes []ComplexType) []TypeInfo {
	out := make([]TypeInfo, len(complexTypes))
	for i := range complexTypes {
		out[i] = NewTypeInfoForComplexType(complexTypes[i])
	}
	return out
}

// EqualTypeInfo reports whether two type info projections expose the same
// runtime-facing facts.
func EqualTypeInfo(a, b TypeInfo) bool {
	return a == b
}

// EqualTypeInfoForComplexType reports whether info exposes the start
// projection for ct.
func EqualTypeInfoForComplexType(info TypeInfo, ct ComplexType) bool {
	return EqualTypeInfo(info, NewTypeInfoForComplexType(ct))
}

// EqualTypeInfosForComplexTypes reports whether infos exposes the start
// projections for complexTypes.
func EqualTypeInfosForComplexTypes(infos []TypeInfo, complexTypes []ComplexType) bool {
	if len(infos) != len(complexTypes) {
		return false
	}
	for len(infos) > 0 {
		if !EqualTypeInfoForComplexType(infos[0], complexTypes[0]) {
			return false
		}
		infos = infos[1:]
		complexTypes = complexTypes[1:]
	}
	return true
}

// ValidateTypeInfosForComplexTypes validates a type-info projection table
// against runtime complex types.
func ValidateTypeInfosForComplexTypes(infos []TypeInfo, complexTypes []ComplexType) error {
	if len(infos) != len(complexTypes) {
		return errors.New("complex type info projection count does not match types")
	}
	if !EqualTypeInfosForComplexTypes(infos, complexTypes) {
		return errors.New("complex type info projection does not match complex type")
	}
	return nil
}

// TypeInfoByID returns validation start data for a runtime type from the
// frozen type-info projection table.
func TypeInfoByID(simpleTypeCount int, infos []TypeInfo, id TypeID) (TypeInfo, bool) {
	if !ValidTypeID(id, simpleTypeCount, len(infos)) {
		return TypeInfo{}, false
	}
	complexID, ok := id.Complex()
	if !ok {
		return TypeInfo{}, true
	}
	if !ValidComplexTypeID(complexID, len(infos)) {
		return TypeInfo{}, false
	}
	return infos[complexID], true
}
