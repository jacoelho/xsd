package types

import "sync"

var builtinCacheOnce sync.Once

// PrecomputeBuiltinCaches initializes built-in type caches for safe concurrent use.
func PrecomputeBuiltinCaches() {
	builtinCacheOnce.Do(func() {
		for _, builtin := range defaultBuiltinRegistry.all() {
			if builtin == nil {
				continue
			}
			builtin.PrimitiveType()
			builtin.FundamentalFacets()
		}
	})
}

// PrecomputeSimpleTypeCaches initializes caches for a simple type.
func PrecomputeSimpleTypeCaches(simpleType *SimpleType) {
	if simpleType == nil {
		return
	}
	simpleType.precomputeCaches()
}
