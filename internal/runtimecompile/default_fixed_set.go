package runtimecompile

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

type defaultFixedTable[K comparable] struct {
	value  map[K]runtime.ValueRef
	key    map[K]runtime.ValueKeyRef
	member map[K]runtime.ValidatorID
}

func newDefaultFixedTable[K comparable]() defaultFixedTable[K] {
	return defaultFixedTable[K]{
		value:  make(map[K]runtime.ValueRef),
		key:    make(map[K]runtime.ValueKeyRef),
		member: make(map[K]runtime.ValidatorID),
	}
}

func (t *defaultFixedTable[K]) store(mapKey K, value compiledDefaultFixed) {
	if t == nil || !value.ok {
		return
	}
	t.value[mapKey] = value.ref
	t.key[mapKey] = value.key
	if value.member != 0 {
		t.member[mapKey] = value.member
	}
}

func (t *defaultFixedTable[K]) lookup(mapKey K) (compiledDefaultFixed, bool) {
	if t == nil {
		return compiledDefaultFixed{}, false
	}
	ref, ok := t.value[mapKey]
	if !ok {
		return compiledDefaultFixed{}, false
	}
	out := compiledDefaultFixed{
		ok:  true,
		ref: ref,
	}
	if key, ok := t.key[mapKey]; ok {
		out.key = key
	}
	if member, ok := t.member[mapKey]; ok {
		out.member = member
	}
	return out, true
}

func (t *defaultFixedTable[K]) contains(mapKey K) bool {
	if t == nil {
		return false
	}
	_, ok := t.value[mapKey]
	return ok
}

type defaultFixedSet[K comparable] struct {
	defaults defaultFixedTable[K]
	fixed    defaultFixedTable[K]
}

func newDefaultFixedSet[K comparable](defaults, fixed defaultFixedTable[K]) defaultFixedSet[K] {
	return defaultFixedSet[K]{
		defaults: defaults,
		fixed:    fixed,
	}
}

func (s *defaultFixedSet[K]) defaultValue(mapKey K) (compiledDefaultFixed, bool) {
	if s == nil {
		return compiledDefaultFixed{}, false
	}
	return s.defaults.lookup(mapKey)
}

func (s *defaultFixedSet[K]) fixedValue(mapKey K) (compiledDefaultFixed, bool) {
	if s == nil {
		return compiledDefaultFixed{}, false
	}
	return s.fixed.lookup(mapKey)
}

func (s *defaultFixedSet[K]) storeDefault(mapKey K, value compiledDefaultFixed) {
	if s == nil {
		return
	}
	s.defaults.store(mapKey, value)
}

func (s *defaultFixedSet[K]) storeFixed(mapKey K, value compiledDefaultFixed) {
	if s == nil {
		return
	}
	s.fixed.store(mapKey, value)
}

func (s *defaultFixedSet[K]) containsDefault(mapKey K) bool {
	if s == nil {
		return false
	}
	return s.defaults.contains(mapKey)
}

func (s *defaultFixedSet[K]) containsFixed(mapKey K) bool {
	if s == nil {
		return false
	}
	return s.fixed.contains(mapKey)
}
