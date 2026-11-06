package validate

import (
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

// RecordAttributes records any XSI schema-location hints in attrs.
func (h *schemaLocationHintSet) RecordAttributes(attrs []xmlstream.Attr, reader *xmlstream.Reader, limits schemaLocationHintLimits, ctx startContext) error {
	var delta schemaLocationHintDelta
	for i := range attrs {
		attr := &attrs[i]
		if !isSchemaLocationHintName(attr.Name) {
			continue
		}
		value, err := reader.AttrValue(attr)
		if err != nil {
			return xsderrors.InternalInvariant("instance reader skipped attribute value")
		}
		if err := h.stageAttribute(attr.Name, value, limits, ctx, &delta); err != nil {
			return err
		}
	}
	if len(delta.namespaces) == 0 {
		return nil
	}
	working := cloneSchemaLocationHints(*h)
	working.commitNamespaceHints(delta.namespaces, delta.namespaceBytes)
	*h = working
	return nil
}
