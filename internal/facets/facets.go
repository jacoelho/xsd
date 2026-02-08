package facets

import "github.com/jacoelho/xsd/internal/types"

// SchemaInput carries schema-time value/facet validation inputs.
type SchemaInput struct {
	Context  map[string]string
	Value    string
	BaseType types.Type
	Facets   []types.Facet
}

// RuntimeInput carries runtime value/facet validation inputs.
type RuntimeInput struct {
	Context  map[string]string
	Value    string
	BaseType types.Type
	Facets   []types.Facet
}

// ValidateSchemaValue validates a value against facets during schema processing.
func ValidateSchemaValue(in SchemaInput) error {
	return types.ValidateValueAgainstFacets(in.Value, in.BaseType, in.Facets, in.Context)
}

// ValidateRuntimeValue validates a value against facets during runtime evaluation.
func ValidateRuntimeValue(in RuntimeInput) error {
	return types.ValidateValueAgainstFacets(in.Value, in.BaseType, in.Facets, in.Context)
}
