package validate

import (
	"encoding/xml"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/uriref"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// SchemaLocationHints records instance schema-location hints seen during
// validation. Only namespaces are retained because loading external schemas is
// intentionally unsupported at validation time.
type SchemaLocationHints struct {
	namespaces     map[string]struct{}
	namespaceBytes int64
}

type schemaLocationHintLimits struct {
	Namespaces     int
	NamespaceBytes int64
}

// RecordAttribute records one xsi:schemaLocation or
// xsi:noNamespaceSchemaLocation attribute value.
func (h *SchemaLocationHints) RecordAttribute(name xml.Name, value string, limits schemaLocationHintLimits, ctx StartContext) error {
	switch name.Local {
	case vocab.XSIAttrSchemaLocation:
		return h.recordNamespaceSchemaLocation(value, limits, ctx)
	case vocab.XSIAttrNoNamespaceSchemaLocation:
		return h.recordNoNamespaceSchemaLocation(value, limits, ctx)
	default:
		return nil
	}
}

// Has reports whether a schema-location hint was seen for ns.
func (h *SchemaLocationHints) Has(ns string) bool {
	if h == nil {
		return false
	}
	_, ok := h.namespaces[ns]
	return ok
}

// Reset clears retained hints, keeping the namespace map only when bounded.
func (h *SchemaLocationHints) Reset(maxRetainedNamespaces int) {
	if h == nil {
		return
	}
	h.namespaceBytes = 0
	if len(h.namespaces) > maxRetainedNamespaces {
		h.namespaces = nil
		return
	}
	clear(h.namespaces)
}

func (h *SchemaLocationHints) recordNamespaceSchemaLocation(value string, limits schemaLocationHintLimits, ctx StartContext) error {
	count := 0
	for field := range lex.XMLFieldsSeq(value) {
		if _, err := uriref.Check(field); err != nil {
			return validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:schemaLocation URI "+field)
		}
		count++
	}
	if count%2 != 0 {
		return validation(ctx, xsderrors.CodeValidationAttribute, "xsi:schemaLocation must contain namespace/location pairs")
	}
	pending := make(map[string]struct{}, min(count/2, limits.Namespaces))
	var pendingBytes int64
	index := 0
	for field := range lex.XMLFieldsSeq(value) {
		if index%2 == 0 {
			if _, exists := h.namespaces[field]; exists {
				index++
				continue
			}
			if _, exists := pending[field]; exists {
				index++
				continue
			}
			if len(h.namespaces)+len(pending) >= limits.Namespaces {
				return validation(ctx, xsderrors.CodeValidationLimit, "schema-location namespace limit exceeded")
			}
			fieldBytes := int64(len(field))
			remaining := limits.NamespaceBytes - h.namespaceBytes
			if remaining < pendingBytes || fieldBytes > remaining-pendingBytes {
				return validation(ctx, xsderrors.CodeValidationLimit, "schema-location namespace byte limit exceeded")
			}
			pending[field] = struct{}{}
			pendingBytes += fieldBytes
		}
		index++
	}
	if len(pending) != 0 && h.namespaces == nil {
		h.namespaces = make(map[string]struct{}, len(pending))
	}
	for ns := range pending {
		h.namespaces[strings.Clone(ns)] = struct{}{}
	}
	h.namespaceBytes += pendingBytes
	return nil
}

func (h *SchemaLocationHints) recordNoNamespaceSchemaLocation(value string, limits schemaLocationHintLimits, ctx StartContext) error {
	value = lex.TrimXMLWhitespaceString(value)
	if _, err := uriref.Check(value); err != nil {
		return validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:noNamespaceSchemaLocation URI "+value)
	}
	return h.add("", limits, ctx)
}

func (h *SchemaLocationHints) add(ns string, limits schemaLocationHintLimits, ctx StartContext) error {
	if _, exists := h.namespaces[ns]; exists {
		return nil
	}
	if len(h.namespaces) >= limits.Namespaces {
		return validation(ctx, xsderrors.CodeValidationLimit, "schema-location namespace limit exceeded")
	}
	nsBytes := int64(len(ns))
	if h.namespaceBytes > limits.NamespaceBytes || nsBytes > limits.NamespaceBytes-h.namespaceBytes {
		return validation(ctx, xsderrors.CodeValidationLimit, "schema-location namespace byte limit exceeded")
	}
	if h.namespaces == nil {
		h.namespaces = make(map[string]struct{})
	}
	h.namespaces[strings.Clone(ns)] = struct{}{}
	h.namespaceBytes += nsBytes
	return nil
}

// IsSchemaLocationHintName reports whether name is an XSI schema-location hint.
func IsSchemaLocationHintName(name xml.Name) bool {
	return name.Space == vocab.XSINamespaceURI &&
		(name.Local == vocab.XSIAttrSchemaLocation || name.Local == vocab.XSIAttrNoNamespaceSchemaLocation)
}
