package validate

import (
	"encoding/xml"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// SchemaLocationHints records instance schema-location hints seen during
// validation. Only namespaces are retained because loading external schemas is
// intentionally unsupported at validation time.
type SchemaLocationHints struct {
	namespaces map[string]bool
}

// RecordAttribute records one xsi:schemaLocation or
// xsi:noNamespaceSchemaLocation attribute value.
func (h *SchemaLocationHints) RecordAttribute(name xml.Name, value string, ctx StartContext) error {
	switch name.Local {
	case vocab.XSIAttrSchemaLocation:
		return h.recordNamespaceSchemaLocation(value, ctx)
	case vocab.XSIAttrNoNamespaceSchemaLocation:
		return h.recordNoNamespaceSchemaLocation(value, ctx)
	default:
		return nil
	}
}

// Has reports whether a schema-location hint was seen for ns.
func (h *SchemaLocationHints) Has(ns string) bool {
	return h != nil && h.namespaces != nil && h.namespaces[ns]
}

// Reset clears retained hints, keeping the namespace map only when bounded.
func (h *SchemaLocationHints) Reset(maxRetainedNamespaces int) {
	if h == nil {
		return
	}
	if len(h.namespaces) > maxRetainedNamespaces {
		h.namespaces = nil
		return
	}
	clear(h.namespaces)
}

func (h *SchemaLocationHints) recordNamespaceSchemaLocation(value string, ctx StartContext) error {
	count := 0
	var err error
	for field := range lex.XMLFieldsSeq(value) {
		if !isAnyURI(field) {
			err = validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:schemaLocation URI "+field)
			break
		}
		count++
	}
	if err != nil {
		return err
	}
	if count%2 != 0 {
		return validation(ctx, xsderrors.CodeValidationAttribute, "xsi:schemaLocation must contain namespace/location pairs")
	}
	index := 0
	for field := range lex.XMLFieldsSeq(value) {
		if index%2 == 0 {
			h.add(field)
		}
		index++
	}
	return nil
}

func (h *SchemaLocationHints) recordNoNamespaceSchemaLocation(value string, ctx StartContext) error {
	value = lex.TrimXMLWhitespaceString(value)
	if value == "" {
		return validation(ctx, xsderrors.CodeValidationAttribute, "xsi:noNamespaceSchemaLocation is empty")
	}
	if !isAnyURI(value) {
		return validation(ctx, xsderrors.CodeValidationAttribute, "invalid xsi:noNamespaceSchemaLocation URI "+value)
	}
	h.add("")
	return nil
}

func (h *SchemaLocationHints) add(ns string) {
	if h.namespaces == nil {
		h.namespaces = make(map[string]bool)
	}
	h.namespaces[ns] = true
}

// IsSchemaLocationHintName reports whether name is an XSI schema-location hint.
func IsSchemaLocationHintName(name xml.Name) bool {
	return name.Space == vocab.XSINamespaceURI &&
		(name.Local == vocab.XSIAttrSchemaLocation || name.Local == vocab.XSIAttrNoNamespaceSchemaLocation)
}

func isAnyURI(s string) bool {
	if s == "" {
		return true
	}
	if s[0] == ':' || s[len(s)-1] == ':' {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '^':
			return false
		case '%':
			if i+2 >= len(s) || !isHexDigit(s[i+1]) || !isHexDigit(s[i+2]) {
				return false
			}
			i += 2
		}
	}
	return true
}

func isHexDigit(b byte) bool {
	return '0' <= b && b <= '9' || 'a' <= b && b <= 'f' || 'A' <= b && b <= 'F'
}
