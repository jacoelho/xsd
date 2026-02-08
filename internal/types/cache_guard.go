package types

import "sync"

type cacheGuard struct {
	cond *sync.Cond
	mu   sync.RWMutex
	once sync.Once
}

func (g *cacheGuard) ensure() {
	g.once.Do(func() {
		g.cond = sync.NewCond(&g.mu)
	})
}

func (b *BuiltinType) guard() *cacheGuard {
	if b == nil {
		return nil
	}
	b.cacheGuard.ensure()
	return &b.cacheGuard
}

func (s *SimpleType) guard() *cacheGuard {
	if s == nil {
		return nil
	}
	s.cacheGuard.ensure()
	return &s.cacheGuard
}
