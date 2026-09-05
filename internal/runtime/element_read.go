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
	identityCount, constraintCount := elementReadStorageCounts(decls)
	table := elementReadTable{
		names:       make([]QName, len(decls)),
		meta:        make([]elementReadMeta, len(decls)),
		identities:  make([]IdentityConstraintID, 0, identityCount),
		constraints: make([]elementConstraintRead, 0, constraintCount),
	}
	for i := range decls {
		table.addDeclaration(i, decls[i], complexTypes)
	}
	return table
}

func elementReadStorageCounts(decls []ElementDecl) (identityCount, constraintCount int) {
	for i := range decls {
		identityCount += len(decls[i].Identity)
		if decls[i].Fixed != nil || decls[i].Default != nil {
			constraintCount++
		}
	}
	return identityCount, constraintCount
}

func (t *elementReadTable) addDeclaration(index int, decl ElementDecl, complexTypes []ComplexType) {
	t.names[index] = decl.Name
	meta := elementReadMeta{
		typ:           decl.Type,
		block:         effectiveElementBlock(decl, complexTypes),
		identityStart: len(t.identities),
		identityCount: len(decl.Identity),
		constraint:    -1,
		flags:         elementFlags(decl),
	}
	t.identities = append(t.identities, decl.Identity...)
	meta.constraint = t.addValueConstraint(decl)
	t.meta[index] = meta
}

func elementFlags(decl ElementDecl) elementReadFlags {
	var flags elementReadFlags
	if decl.Abstract {
		flags |= elementReadAbstract
	}
	if decl.Nillable {
		flags |= elementReadNillable
	}
	return flags
}

func (t *elementReadTable) addValueConstraint(decl ElementDecl) int {
	if decl.Fixed != nil {
		value, _ := newValueConstraintReadFromConstraint(decl.Fixed)
		t.constraints = append(t.constraints, elementConstraintRead{value: value, fixed: true})
		return len(t.constraints) - 1
	}
	if decl.Default != nil {
		value, _ := newValueConstraintReadFromConstraint(decl.Default)
		t.constraints = append(t.constraints, elementConstraintRead{value: value})
		return len(t.constraints) - 1
	}
	return -1
}

func (t *elementReadTable) name(id ElementID) (QName, bool) {
	if !ValidElementID(id, len(t.meta)) || len(t.names) != len(t.meta) {
		return QName{}, false
	}
	return t.names[id], true
}

func (t *elementReadTable) start(id ElementID) (ElementStartInfo, bool) {
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

func (t *elementReadTable) identityConstraints(id ElementID) (IdentityConstraintIDs, bool) {
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

func (t *elementReadTable) valueConstraints(id ElementID) (constraints ElementValueConstraints, present, valid bool) {
	if id == NoElement {
		return ElementValueConstraints{}, false, true
	}
	if !ValidElementID(id, len(t.meta)) {
		return ElementValueConstraints{}, false, false
	}
	meta := t.meta[id]
	if meta.constraint < 0 {
		return newElementValueConstraints(meta.typ, ValueConstraintRead{}, false, ValueConstraintRead{}, false), true, true
	}
	if meta.constraint >= len(t.constraints) {
		return ElementValueConstraints{}, false, false
	}
	constraint := t.constraints[meta.constraint]
	if constraint.fixed {
		return newElementValueConstraints(meta.typ, constraint.value, true, ValueConstraintRead{}, false), true, true
	}
	return newElementValueConstraints(meta.typ, ValueConstraintRead{}, false, constraint.value, true), true, true
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
	audit := elementReadProjectionAudit{table: table, complexTypes: complexTypes}
	for i := range decls {
		if err := audit.validateDeclaration(i, decls[i]); err != nil {
			return err
		}
	}
	if audit.identityOffset != len(table.identities) || audit.constraintOffset != len(table.constraints) {
		return errors.New("element read table retains unreferenced storage")
	}
	return nil
}

type elementReadProjectionAudit struct {
	table            elementReadTable
	complexTypes     []ComplexType
	identityOffset   int
	constraintOffset int
}

func (a *elementReadProjectionAudit) validateDeclaration(index int, decl ElementDecl) error {
	meta := a.table.meta[index]
	if !elementReadMetadataMatches(a.table.names[index], meta, decl, a.complexTypes) {
		return errors.New("element read table metadata does not match declaration")
	}
	if err := a.validateIdentities(meta, decl.Identity); err != nil {
		return err
	}
	return a.validateValueConstraint(index, meta, decl)
}

func elementReadMetadataMatches(name QName, meta elementReadMeta, decl ElementDecl, complexTypes []ComplexType) bool {
	return name == decl.Name && meta.typ == decl.Type && meta.block == effectiveElementBlock(decl, complexTypes) &&
		(meta.flags&elementReadAbstract != 0) == decl.Abstract &&
		(meta.flags&elementReadNillable != 0) == decl.Nillable &&
		meta.flags & ^(elementReadAbstract|elementReadNillable) == 0
}

func (a *elementReadProjectionAudit) validateIdentities(meta elementReadMeta, identities []IdentityConstraintID) error {
	if meta.identityStart != a.identityOffset || meta.identityCount != len(identities) {
		return errors.New("element read table identity span does not match declaration")
	}
	end := a.identityOffset + len(identities)
	if end > len(a.table.identities) || !slices.Equal(a.table.identities[a.identityOffset:end], identities) {
		return errors.New("element read table identities do not match declaration")
	}
	a.identityOffset = end
	return nil
}

func (a *elementReadProjectionAudit) validateValueConstraint(index int, meta elementReadMeta, decl ElementDecl) error {
	if decl.Fixed == nil && decl.Default == nil {
		if meta.constraint != -1 {
			return errors.New("element read table has unexpected value constraint")
		}
		return nil
	}
	if meta.constraint != a.constraintOffset || a.constraintOffset >= len(a.table.constraints) {
		return errors.New("element read table value constraint index does not match declaration")
	}
	if err := validateElementReadValueConstraint(a.table, index, decl); err != nil {
		return err
	}
	a.constraintOffset++
	return nil
}

func validateElementReadValueConstraint(table elementReadTable, index int, decl ElementDecl) error {
	raw, valid := newRuntimeID(index)
	if !valid {
		return errors.New("element read table index limit exceeded")
	}
	got, _, ok := table.valueConstraints(ElementID(raw))
	if !ok {
		return errors.New("element read table value constraint is invalid")
	}
	shape := elementValueConstraintReadShape(decl)
	want := newElementValueConstraints(shape.Owner, shape.Fixed, shape.HasFixed, shape.Default, shape.HasDefault)
	if !equalElementValueConstraints(got, want) {
		return errors.New("element read table value constraint does not match declaration")
	}
	return nil
}
