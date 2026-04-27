package validator

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

type xsiRole uint8

const (
	xsiNone xsiRole = iota
	xsiType
	xsiNil
	xsiSchemaLocation
	xsiNoNamespaceSchemaLocation
)

var (
	xsiLocalType                      = []byte("type")
	xsiLocalNil                       = []byte("nil")
	xsiLocalSchemaLocation            = []byte("schemaLocation")
	xsiLocalNoNamespaceSchemaLocation = []byte("noNamespaceSchemaLocation")
)

// Classify records attribute classes, duplicate-attribute errors, and xsi values.
func (t *Tracker) Classify(rt *runtime.Schema, input []Start, checkDuplicates bool) (Classification, error) {
	if rt == nil || len(input) == 0 {
		return Classification{}, nil
	}

	out := Classification{Classes: t.prepareClasses(len(input))}
	dup := t.prepareDupState(len(input), checkDuplicates)

	for i := range input {
		if checkDuplicates && hasDuplicateAt(rt, input, i, &dup) && out.DuplicateErr == nil {
			out.DuplicateErr = xsderrors.New(xsderrors.ErrXMLParse, "duplicate attribute")
		}
		if err := classifyAndCapture(rt, &out, &input[i], i); err != nil {
			return Classification{}, err
		}
	}

	t.finalizeDupState(dup)
	return out, nil
}

// ClassifyOne returns the high-level role for one attribute.
func ClassifyOne(rt *runtime.Schema, attr *Start) Class {
	class, _ := classifyRole(rt, attr)
	return class
}

func (t *Tracker) prepareClasses(count int) []Class {
	classes := t.Classes
	if cap(classes) < count {
		classes = make([]Class, count)
	} else {
		classes = classes[:count]
	}
	t.Classes = classes
	return classes
}

type dupState struct {
	table   []SeenEntry
	mask    uint64
	useHash bool
}

func (t *Tracker) prepareDupState(attrCount int, checkDuplicates bool) dupState {
	if !checkDuplicates || attrCount <= SmallDuplicateThreshold {
		return dupState{}
	}

	size := runtime.NextPow2(attrCount * 2)
	table := t.Seen
	if cap(table) < size {
		table = make([]SeenEntry, size)
	} else {
		table = table[:size]
		clear(table)
	}
	return dupState{
		table:   table,
		mask:    uint64(size - 1),
		useHash: true,
	}
}

func (t *Tracker) finalizeDupState(state dupState) {
	if state.useHash {
		t.Seen = state.table
	}
}

func hasDuplicateAt(rt *runtime.Schema, attrs []Start, i int, state *dupState) bool {
	if state == nil || !state.useHash {
		return hasDuplicateAtLinear(rt, attrs, i)
	}
	return hasDuplicateAtHash(rt, attrs, i, state)
}

func hasDuplicateAtLinear(rt *runtime.Schema, attrs []Start, i int) bool {
	for j := range i {
		if namesEqual(rt, &attrs[j], &attrs[i]) {
			return true
		}
	}
	return false
}

func hasDuplicateAtHash(rt *runtime.Schema, attrs []Start, i int, state *dupState) bool {
	if state == nil || !state.useHash {
		return false
	}

	nsBytes := namespaceBytes(rt, &attrs[i])
	hash := NameHash(nsBytes, attrs[i].Local)
	slot := int(hash & state.mask)
	for {
		entry := state.table[slot]
		if entry.Hash == 0 {
			state.table[slot] = SeenEntry{Hash: hash, Index: uint32(i)}
			return false
		}
		if entry.Hash == hash && namesEqual(rt, &attrs[int(entry.Index)], &attrs[i]) {
			return true
		}
		slot = (slot + 1) & int(state.mask)
	}
}

func classifyAndCapture(rt *runtime.Schema, out *Classification, attr *Start, idx int) error {
	if out == nil || attr == nil || idx < 0 || idx >= len(out.Classes) {
		return nil
	}

	class, role := classifyRole(rt, attr)
	out.Classes[idx] = class
	switch role {
	case xsiType:
		if len(out.XSIType) > 0 {
			return xsderrors.New(xsderrors.ErrDatatypeInvalid, "duplicate xsi:type attribute")
		}
		out.XSIType = attr.Value
	case xsiNil:
		if len(out.XSINil) > 0 {
			return xsderrors.New(xsderrors.ErrDatatypeInvalid, "duplicate xsi:nil attribute")
		}
		out.XSINil = attr.Value
	}
	return nil
}

func classifyRole(rt *runtime.Schema, attr *Start) (Class, xsiRole) {
	role := xsiAttributeRole(rt, attr)
	if role != xsiNone {
		return ClassXSIKnown, role
	}
	if isXSINamespaceAttr(rt, attr) {
		return ClassXSIUnknown, xsiNone
	}
	if isXMLAttribute(rt, attr) {
		return ClassXML, xsiNone
	}
	return ClassOther, xsiNone
}

func xsiAttributeRole(rt *runtime.Schema, attr *Start) xsiRole {
	if rt == nil || attr == nil {
		return xsiNone
	}
	predef := rt.KnownSymbols()
	switch {
	case attr.Sym != 0 && predef.XsiType != 0 && attr.Sym == predef.XsiType:
		return xsiType
	case attr.Sym != 0 && predef.XsiNil != 0 && attr.Sym == predef.XsiNil:
		return xsiNil
	case attr.Sym != 0 && predef.XsiSchemaLocation != 0 && attr.Sym == predef.XsiSchemaLocation:
		return xsiSchemaLocation
	case attr.Sym != 0 && predef.XsiNoNamespaceSchemaLocation != 0 && attr.Sym == predef.XsiNoNamespaceSchemaLocation:
		return xsiNoNamespaceSchemaLocation
	}
	if !isXSINamespaceAttr(rt, attr) {
		return xsiNone
	}
	switch {
	case bytes.Equal(attr.Local, xsiLocalType):
		return xsiType
	case bytes.Equal(attr.Local, xsiLocalNil):
		return xsiNil
	case bytes.Equal(attr.Local, xsiLocalSchemaLocation):
		return xsiSchemaLocation
	case bytes.Equal(attr.Local, xsiLocalNoNamespaceSchemaLocation):
		return xsiNoNamespaceSchemaLocation
	default:
		return xsiNone
	}
}

func namesEqual(rt *runtime.Schema, a, b *Start) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Sym != 0 && b.Sym != 0 {
		return a.Sym == b.Sym
	}
	return bytes.Equal(namespaceBytes(rt, a), namespaceBytes(rt, b)) && bytes.Equal(a.Local, b.Local)
}

func namespaceBytes(rt *runtime.Schema, attr *Start) []byte {
	if rt != nil && attr != nil && attr.NS != 0 {
		return rt.NamespaceBytes(attr.NS)
	}
	if attr == nil {
		return nil
	}
	return attr.NSBytes
}

func isXSINamespaceAttr(rt *runtime.Schema, attr *Start) bool {
	if rt == nil || attr == nil {
		return false
	}
	predefNS := rt.KnownNamespaces()
	if attr.NS != 0 {
		return attr.NS == predefNS.Xsi
	}
	target := rt.NamespaceBytes(predefNS.Xsi)
	if len(target) == 0 {
		return false
	}
	return bytes.Equal(target, attr.NSBytes)
}

func isXMLAttribute(rt *runtime.Schema, attr *Start) bool {
	if rt == nil || attr == nil {
		return false
	}
	predefNS := rt.KnownNamespaces()
	if attr.NS != 0 {
		return attr.NS == predefNS.XML
	}
	target := rt.NamespaceBytes(predefNS.XML)
	if len(target) == 0 {
		return false
	}
	return bytes.Equal(target, attr.NSBytes)
}
