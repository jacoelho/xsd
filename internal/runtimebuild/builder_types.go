package runtimebuild

import (
	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

type schemaBuilder struct {
	err            error
	builder        *runtime.Builder
	complexIDs     map[runtime.TypeID]uint32
	schema         *schemair.Schema
	artifacts      runtimeArtifacts
	rt             *runtime.Schema
	paths          []runtime.PathProgram
	wildcards      []runtime.WildcardRule
	wildcardIDs    map[schemair.WildcardID]runtime.WildcardID
	wildcardNS     []runtime.NamespaceID
	notations      []runtime.SymbolID
	maxOccurs      uint32
	anyTypeComplex uint32
	limits         contentmodel.Limits
}

const defaultMaxOccursLimit = 1_000_000
