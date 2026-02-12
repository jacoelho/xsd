package compiler

import (
	"sync"

	"github.com/jacoelho/xsd/internal/normalize"
	"github.com/jacoelho/xsd/internal/runtimeassemble"
)

// BuildConfig configures runtime compilation.
type BuildConfig struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

// Prepared stores normalized artifacts and lazy build state.
type Prepared struct {
	prepErr   error
	artifacts *normalize.Artifacts
	prepared  *runtimeassemble.PreparedArtifacts
	buildOnce sync.Once
	buildMu   sync.Mutex
}
