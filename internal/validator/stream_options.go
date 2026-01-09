package validator

// SchemaLocationPolicy controls how xsi:schemaLocation hints are handled in streaming schemacheck.
type SchemaLocationPolicy int

const (
	// SchemaLocationRootOnly applies only schemaLocation hints found on the root element.
	SchemaLocationRootOnly SchemaLocationPolicy = iota
	// SchemaLocationDocument scans the entire document for schemaLocation hints before schemacheck.
	SchemaLocationDocument
	// SchemaLocationIgnore ignores all schemaLocation hints.
	SchemaLocationIgnore
)

// StreamOptions configures streaming validation behavior.
type StreamOptions struct {
	SchemaLocationPolicy SchemaLocationPolicy
}
