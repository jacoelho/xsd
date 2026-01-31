package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// Path identifies the node location for rich errors.
type Path string

// NodeKind describes where a value was sourced from.
type NodeKind uint8

const (
	NodeUnknown NodeKind = iota
	NodeAttribute
	NodeElementText
)

// NamespaceContext resolves QName prefixes to namespace URIs.
type NamespaceContext = value.NSResolver

// ValueContext carries contextual information for simple value validation.
type ValueContext struct {
	Path     Path
	NodeKind NodeKind
	NS       NamespaceContext
}

// Value contains the validated value metadata.
type Value struct {
	TypeID       runtime.TypeID
	ActualTypeID runtime.TypeID
	Key          runtime.ValueKey
}

// ValidateSimple validates one lexical value against one simple type.
func ValidateSimple(sess *Session, typeID runtime.TypeID, raw []byte, ctx ValueContext) (Value, bool) {
	if sess == nil || sess.rt == nil {
		return Value{}, false
	}
	typ, ok := sess.typeByID(typeID)
	if !ok || typ.Kind == runtime.TypeComplex {
		return Value{}, false
	}
	validator := typ.Validator
	_, metrics, err := sess.validateValueInternalWithMetrics(validator, raw, ctx.NS, valueOptions{
		applyWhitespace:  true,
		trackIDs:         false,
		requireCanonical: true,
		storeValue:       false,
		needKey:          true,
	})
	if err != nil || !metrics.keySet {
		return Value{}, false
	}
	actualType := typeID
	if metrics.actualTypeID != 0 {
		actualType = metrics.actualTypeID
	}
	key := runtime.ValueKey{
		Kind:  metrics.keyKind,
		Hash:  runtime.HashKey(metrics.keyKind, metrics.keyBytes),
		Bytes: metrics.keyBytes,
	}
	return Value{
		TypeID:       typeID,
		ActualTypeID: actualType,
		Key:          FreezeKey(sess, key),
	}, true
}
