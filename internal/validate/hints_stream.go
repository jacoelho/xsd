package validate

import "github.com/jacoelho/xsd/internal/stream"

// RecordAttributes records any XSI schema-location hints in attrs.
func (h *SchemaLocationHints) RecordAttributes(attrs []stream.Attr, values *stream.Cache, limits schemaLocationHintLimits, ctx StartContext) error {
	var delta schemaLocationHintDelta
	for i := range attrs {
		attr := &attrs[i]
		if !IsSchemaLocationHintName(attr.Name) {
			continue
		}
		if err := h.stageAttribute(attr.Name, attr.StringValue(values), limits, ctx, &delta); err != nil {
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
