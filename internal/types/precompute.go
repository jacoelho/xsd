package types

import "sync"

var builtinCacheOnce sync.Once

// PrecomputeBuiltinCaches initializes built-in type caches for safe concurrent use.
func PrecomputeBuiltinCaches() {
	builtinCacheOnce.Do(func() {
		for _, builtin := range builtinRegistry {
			if builtin == nil {
				continue
			}
			_ = builtin.PrimitiveType()
			_ = builtin.FundamentalFacets()
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
