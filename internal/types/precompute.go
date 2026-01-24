package types

import "sync"

var builtinCacheOnce sync.Once

// PrecomputeBuiltinCaches initializes built-in type caches for safe concurrent use.
// Call this before concurrent validation to avoid unsynchronized cache writes.
func PrecomputeBuiltinCaches() {
	builtinCacheOnce.Do(func() {
		for _, builtin := range builtinRegistry {
			if builtin == nil {
				continue
			}
			builtin.primitiveTypeCache = builtin.computePrimitiveType()
			builtin.fundamentalFacetsCache = builtin.FundamentalFacets()
		}
	})
}

// PrecomputeSimpleTypeCaches initializes caches for a simple type.
// Call this before concurrent validation to avoid unsynchronized cache writes.
func PrecomputeSimpleTypeCaches(simpleType *SimpleType) {
	if simpleType == nil {
		return
	}
	simpleType.precomputeCaches()
}
