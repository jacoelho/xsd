package schema_test

import xsdSchema "github.com/jacoelho/xsd/internal/schema"

func unlimitedContentModelWork(int) error { return nil }

func publishSchema(build *xsdSchema.SchemaBuild) (*xsdSchema.Schema, error) {
	return xsdSchema.PublishSchema(build, unlimitedContentModelWork)
}
