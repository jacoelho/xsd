package xsd

import "github.com/jacoelho/xsd/internal/validator"

// SchemaLocationPolicy controls how xsi:schemaLocation hints are handled.
type SchemaLocationPolicy int

const (
	// SchemaLocationRootOnly applies only schemaLocation hints found on the root element.
	SchemaLocationRootOnly SchemaLocationPolicy = iota
	// SchemaLocationDocument scans the entire document for schemaLocation hints before schemacheck.
	SchemaLocationDocument
	// SchemaLocationIgnore ignores all schemaLocation hints.
	SchemaLocationIgnore
)

// ValidateOptions configures validation behavior.
type ValidateOptions struct {
	SchemaLocationPolicy SchemaLocationPolicy
}

func toStreamOptions(opts ValidateOptions) validator.StreamOptions {
	return validator.StreamOptions{
		SchemaLocationPolicy: toValidatorSchemaLocationPolicy(opts.SchemaLocationPolicy),
	}
}

func toValidatorSchemaLocationPolicy(policy SchemaLocationPolicy) validator.SchemaLocationPolicy {
	switch policy {
	case SchemaLocationDocument:
		return validator.SchemaLocationDocument
	case SchemaLocationIgnore:
		return validator.SchemaLocationIgnore
	default:
		return validator.SchemaLocationRootOnly
	}
}
