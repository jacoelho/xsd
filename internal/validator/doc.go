// Package validator validates XML documents against compiled XSD schemas.
// The root package keeps only session/runtime orchestration, while attrs,
// start, flow, facetexec, and the other helper subpackages own the reusable
// leaf subsystems behind the validation pipeline.
package validator
