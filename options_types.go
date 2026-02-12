package xsd

import "github.com/jacoelho/xsd/pkg/xmlstream"

type intOption struct {
	value int
	set   bool
}

func (o intOption) resolved() int {
	if !o.set {
		return 0
	}
	return o.value
}

type uint32Option struct {
	value uint32
	set   bool
}

func (o uint32Option) resolved() uint32 {
	if !o.set {
		return 0
	}
	return o.value
}

// LoadOptions configures schema loading and default runtime compilation.
type LoadOptions struct {
	runtime                     RuntimeOptions
	allowMissingImportLocations bool
	schemaMaxDepth              intOption
	schemaMaxAttrs              intOption
	schemaMaxTokenSize          intOption
	schemaMaxQNameInternEntries intOption
}

// RuntimeOptions configures runtime compilation and instance XML limits.
type RuntimeOptions struct {
	maxDFAStates                  uint32Option
	maxOccursLimit                uint32Option
	instanceMaxDepth              intOption
	instanceMaxAttrs              intOption
	instanceMaxTokenSize          intOption
	instanceMaxQNameInternEntries intOption
}

type resolvedLoadOptions struct {
	schemaLimits                xmlParseLimits
	allowMissingImportLocations bool
}

type resolvedRuntimeOptions struct {
	instanceParseOptions []xmlstream.Option
	instanceLimits       xmlParseLimits
	maxDFAStates         uint32
	maxOccursLimit       uint32
}
