package validate

import "github.com/jacoelho/xsd/internal/stream"

// RecordAttributes records any XSI schema-location hints in attrs.
func (h *SchemaLocationHints) RecordAttributes(attrs []stream.Attr, values *stream.Cache, limits schemaLocationHintLimits, ctx StartContext) error {
	working := cloneSchemaLocationHints(*h)
	for i := range attrs {
		attr := &attrs[i]
		if !IsSchemaLocationHintName(attr.Name) {
			continue
		}
		if err := working.RecordAttribute(attr.Name, attr.StringValue(values), limits, ctx); err != nil {
			return err
		}
	}
	*h = working
	return nil
}
