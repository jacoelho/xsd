package runtime

import "slices"

// ElementIdentityConstraintReadShape is the source projection for the identity
// constraints attached to one element declaration.
type ElementIdentityConstraintReadShape struct {
	Identity []IdentityConstraintID
}

// NewDeclaredIdentityConstraint constructs a declared but not yet compiled
// identity constraint placeholder.
func NewDeclaredIdentityConstraint(name QName) IdentityConstraint {
	return IdentityConstraint{Name: name, Refer: NoIdentityConstraint}
}

// NewIdentityConstraint constructs identity metadata and owns the derived field
// lookup tables used at validation time.
func NewIdentityConstraint(kind IdentityKind, name QName, refer IdentityConstraintID, selector []IdentityPath, fields []IdentityField) IdentityConstraint {
	if kind != IdentityKeyRef {
		refer = NoIdentityConstraint
	}
	selector = CloneIdentityPaths(selector)
	fields = CloneIdentityFields(fields)
	elementFields, attrFields, attrWildcardFields := BuildIdentityFieldLookup(fields)
	return IdentityConstraint{
		Selector:                selector,
		Fields:                  fields,
		ElementFields:           elementFields,
		AttributeFields:         attrFields,
		AttributeWildcardFields: attrWildcardFields,
		Name:                    name,
		Refer:                   refer,
		Kind:                    kind,
	}
}

// CloneIdentityConstraintIDs clones an identity-constraint ID list for frozen
// runtime publication.
func CloneIdentityConstraintIDs(in []IdentityConstraintID) []IdentityConstraintID {
	return slices.Clone(in)
}

// NewElementIdentityConstraintReads clones per-element identity constraint IDs
// for frozen runtime publication.
func NewElementIdentityConstraintReads(shapes []ElementIdentityConstraintReadShape) [][]IdentityConstraintID {
	out := make([][]IdentityConstraintID, len(shapes))
	for i := range shapes {
		out[i] = CloneIdentityConstraintIDs(shapes[i].Identity)
	}
	return out
}

// NewElementIdentityConstraintReadsForDecls clones per-element identity
// constraint IDs from frozen element declarations for runtime publication.
func NewElementIdentityConstraintReadsForDecls(decls []ElementDecl) [][]IdentityConstraintID {
	out := make([][]IdentityConstraintID, len(decls))
	for i := range decls {
		out[i] = CloneIdentityConstraintIDs(decls[i].Identity)
	}
	return out
}

func moveElementIdentityConstraintReads(decls []ElementDecl) [][]IdentityConstraintID {
	out := make([][]IdentityConstraintID, len(decls))
	for i := range decls {
		out[i] = decls[i].Identity
	}
	return out
}

// CloneIdentityPaths deep-clones parsed identity selector path metadata.
func CloneIdentityPaths(in []IdentityPath) []IdentityPath {
	out := slices.Clone(in)
	for i := range out {
		out[i].Steps = slices.Clone(in[i].Steps)
	}
	return out
}

// CloneIdentityFields deep-clones parsed identity field metadata.
func CloneIdentityFields(in []IdentityField) []IdentityField {
	out := slices.Clone(in)
	for i := range out {
		out[i].Paths = cloneIdentityFieldPaths(in[i].Paths)
	}
	return out
}

func cloneIdentityFieldPaths(in []IdentityFieldPath) []IdentityFieldPath {
	out := slices.Clone(in)
	for i := range out {
		out[i] = cloneIdentityFieldPath(in[i])
	}
	return out
}

func cloneIdentityFieldPath(in IdentityFieldPath) IdentityFieldPath {
	in.Steps = slices.Clone(in.Steps)
	return in
}

func cloneCompiledIdentityFields(in []CompiledIdentityField) []CompiledIdentityField {
	out := slices.Clone(in)
	for i := range out {
		out[i].Paths = cloneIdentityFieldPaths(in[i].Paths)
	}
	return out
}

func cloneCompiledIdentityFieldMap(in map[QName][]CompiledIdentityField) map[QName][]CompiledIdentityField {
	if in == nil {
		return nil
	}
	out := make(map[QName][]CompiledIdentityField, len(in))
	for name, fields := range in {
		out[name] = cloneCompiledIdentityFields(fields)
	}
	return out
}
