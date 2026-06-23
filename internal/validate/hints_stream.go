package validate

import "github.com/jacoelho/xsd/internal/stream"

// RecordAttributes records any XSI schema-location hints in attrs.
func (h *SchemaLocationHints) RecordAttributes(attrs []stream.Attr, values *stream.Cache, ctx StartContext) error {
	for i := range attrs {
		attr := &attrs[i]
		if !IsSchemaLocationHintName(attr.Name) {
			continue
		}
		if err := h.RecordAttribute(attr.Name, attr.StringValue(values), ctx); err != nil {
			return err
		}
	}
	return nil
}
