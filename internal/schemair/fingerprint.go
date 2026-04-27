package schemair

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"maps"
	"slices"
)

func (s *Schema) Fingerprint() [32]byte {
	h := fingerprintWriter{h: sha256.New()}
	if s == nil {
		return sha256.Sum256(nil)
	}
	h.writeNames(s.Names.Values)
	h.writeBuiltinTypes(s.BuiltinTypes)
	h.writeTypes(s.Types)
	h.writeSimpleTypes(s.SimpleTypes)
	h.writeComplexTypes(s.ComplexTypes)
	h.writeElements(s.Elements)
	h.writeAttributes(s.Attributes)
	h.writeIdentityConstraints(s.IdentityConstraints)
	h.writeElementRefs(s.ElementRefs)
	h.writeAttributeRefs(s.AttributeRefs)
	h.writeGroupRefs(s.GroupRefs)
	h.writeRuntimeNamePlan(s.RuntimeNames)
	h.writeGlobalIndexes(s.GlobalIndexes)
	var out [32]byte
	sum := h.h.Sum(nil)
	copy(out[:], sum)
	return out
}

type fingerprintWriter struct {
	h   hash.Hash
	buf [8]byte
}

func (w *fingerprintWriter) writeU8(v uint8) {
	w.buf[0] = v
	_, _ = w.h.Write(w.buf[:1])
}

func (w *fingerprintWriter) writeU32(v uint32) {
	binary.LittleEndian.PutUint32(w.buf[:4], v)
	_, _ = w.h.Write(w.buf[:4])
}

func (w *fingerprintWriter) writeString(v string) {
	w.writeU32(uint32(len(v)))
	if len(v) == 0 {
		return
	}
	_, _ = w.h.Write([]byte(v))
}

func (w *fingerprintWriter) writeBool(v bool) {
	if v {
		w.writeU8(1)
		return
	}
	w.writeU8(0)
}

func (w *fingerprintWriter) writeName(v Name) {
	w.writeString(v.Namespace)
	w.writeString(v.Local)
}

func (w *fingerprintWriter) writeTypeRef(v TypeRef) {
	w.writeU32(uint32(v.TypeID()))
	w.writeName(v.TypeName())
	w.writeBool(v.IsBuiltin())
}

func (w *fingerprintWriter) writeBuiltinTypes(values []BuiltinType) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeName(value.Name)
		w.writeTypeRef(value.Base)
		w.writeBool(value.AnyType)
		w.writeBool(value.AnySimpleType)
		w.writeSimpleType(value.Value)
	}
}

func (w *fingerprintWriter) writeNames(values []Name) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeName(value)
	}
}

func (w *fingerprintWriter) writeTypes(values []TypeDecl) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeU32(uint32(value.ID))
		w.writeName(value.Name)
		w.writeU8(uint8(value.Kind))
		w.writeTypeRef(value.Base)
		w.writeU8(uint8(value.Derivation))
		w.writeU8(uint8(value.Final))
		w.writeU8(uint8(value.Block))
		w.writeBool(value.Abstract)
		w.writeBool(value.Global)
		w.writeString(value.Origin)
	}
}

func (w *fingerprintWriter) writeElements(values []Element) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeU32(uint32(value.ID))
		w.writeName(value.Name)
		w.writeTypeRef(value.TypeDecl)
		w.writeU32(uint32(value.SubstitutionHead))
		w.writeValueConstraint(value.Default)
		w.writeValueConstraint(value.Fixed)
		w.writeU8(uint8(value.Final))
		w.writeU8(uint8(value.Block))
		w.writeBool(value.Nillable)
		w.writeBool(value.Abstract)
		w.writeBool(value.Global)
		w.writeString(value.Origin)
	}
}

func (w *fingerprintWriter) writeAttributes(values []Attribute) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeU32(uint32(value.ID))
		w.writeName(value.Name)
		w.writeTypeRef(value.TypeDecl)
		w.writeValueConstraint(value.Default)
		w.writeValueConstraint(value.Fixed)
		w.writeBool(value.Global)
		w.writeString(value.Origin)
	}
}

func (w *fingerprintWriter) writeSimpleTypes(values []SimpleTypeSpec) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeSimpleType(value)
	}
}

func (w *fingerprintWriter) writeSimpleType(value SimpleTypeSpec) {
	w.writeU32(uint32(value.TypeDecl))
	w.writeName(value.Name)
	w.writeBool(value.Builtin)
	w.writeU8(uint8(value.Variety))
	w.writeTypeRef(value.Base)
	w.writeTypeRef(value.Item)
	w.writeU32(uint32(len(value.Members)))
	for _, member := range value.Members {
		w.writeTypeRef(member)
	}
	w.writeU32(uint32(len(value.Facets)))
	for _, facet := range value.Facets {
		w.writeFacet(facet)
	}
	w.writeString(value.Primitive)
	w.writeString(value.BuiltinBase)
	w.writeU8(uint8(value.Whitespace))
	w.writeBool(value.QNameOrNotation)
	w.writeBool(value.IntegerDerived)
}

func (w *fingerprintWriter) writeFacet(value FacetSpec) {
	w.writeU8(uint8(value.Kind))
	w.writeString(value.Name)
	w.writeString(value.Value)
	w.writeU32(value.IntValue)
	w.writeU32(uint32(len(value.Values)))
	for _, item := range value.Values {
		w.writeString(item.Lexical)
		w.writeStringMap(item.Context)
	}
}

func (w *fingerprintWriter) writeComplexTypes(values []ComplexTypePlan) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeU32(uint32(value.TypeDecl))
		w.writeBool(value.Mixed)
		w.writeU8(uint8(value.Content))
		w.writeTypeRef(value.TextType)
		w.writeSimpleType(value.TextSpec)
		w.writeU32(uint32(len(value.Attrs)))
		for _, attr := range value.Attrs {
			w.writeU32(uint32(attr))
		}
		w.writeU32(uint32(value.AnyAttr))
		w.writeU32(uint32(value.Particle))
	}
}

func (w *fingerprintWriter) writeValueConstraint(value ValueConstraint) {
	w.writeBool(value.IsPresent())
	w.writeString(value.LexicalValue())
	w.writeStringMap(value.NamespaceContext())
}

func (w *fingerprintWriter) writeStringMap(values map[string]string) {
	w.writeU32(uint32(len(values)))
	for _, key := range slices.Sorted(maps.Keys(values)) {
		w.writeString(key)
		w.writeString(values[key])
	}
}

func (w *fingerprintWriter) writeIdentityConstraints(values []IdentityConstraint) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeU32(uint32(value.ID))
		w.writeU32(uint32(value.Element))
		w.writeName(value.Name)
		w.writeU8(uint8(value.Kind))
		w.writeString(value.Selector)
		w.writeU32(uint32(len(value.Fields)))
		for _, field := range value.Fields {
			w.writeString(field.XPath)
			w.writeTypeRef(field.TypeDecl)
		}
		w.writeName(value.Refer)
	}
}

func (w *fingerprintWriter) writeElementRefs(values []ElementReference) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeName(value.Name)
		w.writeU32(uint32(value.Element))
	}
}

func (w *fingerprintWriter) writeAttributeRefs(values []AttributeReference) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeName(value.Name)
		w.writeU32(uint32(value.Attribute))
	}
}

func (w *fingerprintWriter) writeGroupRefs(values []GroupReference) {
	w.writeU32(uint32(len(values)))
	for _, value := range values {
		w.writeName(value.Name)
		w.writeName(value.Target)
	}
}

func (w *fingerprintWriter) writeRuntimeNamePlan(plan RuntimeNamePlan) {
	w.writeU32(uint32(len(plan.Ops)))
	for _, op := range plan.Ops {
		w.writeU8(uint8(op.Kind))
		w.writeName(op.Name)
		w.writeString(op.Namespace)
	}
	w.writeNames(plan.Notations)
}

func (w *fingerprintWriter) writeGlobalIndexes(indexes GlobalIndexes) {
	w.writeU32(uint32(len(indexes.Types)))
	for _, value := range indexes.Types {
		w.writeName(value.Name)
		w.writeU32(uint32(value.TypeDecl))
		w.writeBool(value.Builtin)
	}
	w.writeU32(uint32(len(indexes.Elements)))
	for _, value := range indexes.Elements {
		w.writeName(value.Name)
		w.writeU32(uint32(value.Element))
	}
	w.writeU32(uint32(len(indexes.Attributes)))
	for _, value := range indexes.Attributes {
		w.writeName(value.Name)
		w.writeU32(uint32(value.Attribute))
	}
}
