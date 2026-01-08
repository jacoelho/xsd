package validator

// SchemaLocationPolicy controls how xsi:schemaLocation hints are handled in streaming validation.
type SchemaLocationPolicy int

const (
	SchemaLocationError SchemaLocationPolicy = iota
	SchemaLocationIgnore
)

// StreamOptions configures streaming validation behavior.
type StreamOptions struct {
	SchemaLocationPolicy SchemaLocationPolicy
}
