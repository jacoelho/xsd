package runtime

import (
	"errors"
	"slices"
)

type elementReadFlags uint8

const (
	elementReadAbstract elementReadFlags = 1 << iota
	elementReadNillable
)

type elementReadMeta struct {
	typ           TypeID
	identityStart int
	identityCount int
	constraint    int
	block         DerivationMask
	flags         elementReadFlags
}

type elementConstraintRead struct {
	value ValueConstraintRead
	fixed bool
}

// elementReadTable is the sole published owner of element declaration facts.
// Names stay columnar because content-model matching reads them without the
// colder start, identity, and value-constraint metadata.
type elementReadTable struct {
	names       []QName
	meta        []elementReadMeta
	identities  []IdentityConstraintID
	constraints []elementConstraintRead
}

func newElementReadTable(decls []ElementDecl, complexTypes []ComplexType) elementReadTable {
	identityCount := 0
	constraintCount := 0
	for i := range decls {
		identityCount += len(decls[i].Identity)
		if decls[i].Fixed != nil || decls[i].Default != nil {
			constraintCount++
		}
	}
	table := elementReadTable{
		names:       make([]QName, len(decls)),
		meta:        make([]elementReadMeta, len(decls)),
		identities:  make([]IdentityConstraintID, 0, identityCount),
		constraints: make([]elementConstraintRead, 0, constraintCount),
	}
	for i := range decls {
		decl := &decls[i]
		table.names[i] = decl.Name
		meta := elementReadMeta{
			typ:           decl.Type,
			block:         effectiveElementBlock(*decl, complexTypes),
			identityStart: len(table.identities),
			identityCount: len(decl.Identity),
			constraint:    -1,
		}
		if decl.Abstract {
			meta.flags |= elementReadAbstract
		}
		if decl.Nillable {
			meta.flags |= elementReadNillable
		}
		table.identities = append(table.identities, decl.Identity...)
		if decl.Fixed != nil {
			value, _ := NewValueConstraintReadFromConstraint(decl.Fixed)
			meta.constraint = len(table.constraints)
			table.constraints = append(table.constraints, elementConstraintRead{value: value, fixed: true})
		} else if decl.Default != nil {
			value, _ := NewValueConstraintReadFromConstraint(decl.Default)
			meta.constraint = len(table.constraints)
			table.constraints = append(table.constraints, elementConstraintRead{value: value})
		}
		table.meta[i] = meta
	}
	return table
}

func (t elementReadTable) len() int {
	return len(t.meta)
}

func (t elementReadTable) name(id ElementID) (QName, bool) {
	if !ValidElementID(id, len(t.meta)) || len(t.names) != len(t.meta) {
		return QName{}, false
	}
	return t.names[id], true
}

func (t elementReadTable) start(id ElementID) (ElementStartInfo, bool) {
	if !ValidElementID(id, len(t.meta)) || len(t.names) != len(t.meta) {
		return ElementStartInfo{}, false
	}
	meta := t.meta[id]
	fixed, def := false, false
	if meta.constraint >= 0 {
		if meta.constraint >= len(t.constraints) {
			return ElementStartInfo{}, false
		}
		fixed = t.constraints[meta.constraint].fixed
		def = !fixed
	}
	return ElementStartInfo{
		Type:     meta.typ,
		Block:    meta.block,
		Abstract: meta.flags&elementReadAbstract != 0,
		Nillable: meta.flags&elementReadNillable != 0,
		Fixed:    fixed,
		Default:  def,
	}, true
}

func (t elementReadTable) identityConstraints(id ElementID) (IdentityConstraintIDs, bool) {
	if !ValidElementID(id, len(t.meta)) {
		return IdentityConstraintIDs{}, false
	}
	meta := t.meta[id]
	end := meta.identityStart + meta.identityCount
	if meta.identityStart < 0 || meta.identityCount < 0 || end < meta.identityStart || end > len(t.identities) {
		return IdentityConstraintIDs{}, false
	}
	return borrowedIdentityConstraintIDs(t.identities[meta.identityStart:end]), true
}

func (t elementReadTable) valueConstraints(id ElementID) (ElementValueConstraints, bool, bool) {
	if id == NoElement {
		return ElementValueConstraints{}, false, true
	}
	if !ValidElementID(id, len(t.meta)) {
		return ElementValueConstraints{}, false, false
	}
	meta := t.meta[id]
	if meta.constraint < 0 {
		return NewElementValueConstraints(meta.typ, ValueConstraintRead{}, false, ValueConstraintRead{}, false), true, true
	}
	if meta.constraint >= len(t.constraints) {
		return ElementValueConstraints{}, false, false
	}
	constraint := t.constraints[meta.constraint]
	if constraint.fixed {
		return NewElementValueConstraints(meta.typ, constraint.value, true, ValueConstraintRead{}, false), true, true
	}
	return NewElementValueConstraints(meta.typ, ValueConstraintRead{}, false, constraint.value, true), true, true
}

func effectiveElementBlock(decl ElementDecl, complexTypes []ComplexType) DerivationMask {
	block := decl.Block
	if id, ok := decl.Type.Complex(); ok && ValidComplexTypeID(id, len(complexTypes)) {
		block |= complexTypes[id].Block
	}
	return block
}

func validateElementReadTableProjection(table elementReadTable, decls []ElementDecl, complexTypes []ComplexType) error {
	if len(table.names) != len(decls) || len(table.meta) != len(decls) {
		return errors.New("element read table count does not match declarations")
	}
	identityOffset := 0
	constraintOffset := 0
	for i := range decls {
		decl := &decls[i]
		meta := table.meta[i]
		if table.names[i] != decl.Name || meta.typ != decl.Type || meta.block != effectiveElementBlock(*decl, complexTypes) ||
			(meta.flags&elementReadAbstract != 0) != decl.Abstract ||
			(meta.flags&elementReadNillable != 0) != decl.Nillable ||
			meta.flags & ^(elementReadAbstract|elementReadNillable) != 0 {
			return errors.New("element read table metadata does not match declaration")
		}
		if meta.identityStart != identityOffset || meta.identityCount != len(decl.Identity) {
			return errors.New("element read table identity span does not match declaration")
		}
		end := identityOffset + len(decl.Identity)
		if end > len(table.identities) || !slices.Equal(table.identities[identityOffset:end], decl.Identity) {
			return errors.New("element read table identities do not match declaration")
		}
		identityOffset = end
		hasConstraint := decl.Fixed != nil || decl.Default != nil
		if !hasConstraint {
			if meta.constraint != -1 {
				return errors.New("element read table has unexpected value constraint")
			}
			continue
		}
		if meta.constraint != constraintOffset || constraintOffset >= len(table.constraints) {
			return errors.New("element read table value constraint index does not match declaration")
		}
		got, _, ok := table.valueConstraints(ElementID(i))
		if !ok {
			return errors.New("element read table value constraint is invalid")
		}
		shape := elementValueConstraintReadShape(*decl)
		want := NewElementValueConstraints(shape.Owner, shape.Fixed, shape.HasFixed, shape.Default, shape.HasDefault)
		if !EqualElementValueConstraints(got, want) {
			return errors.New("element read table value constraint does not match declaration")
		}
		constraintOffset++
	}
	if identityOffset != len(table.identities) || constraintOffset != len(table.constraints) {
		return errors.New("element read table retains unreferenced storage")
	}
	return nil
}
