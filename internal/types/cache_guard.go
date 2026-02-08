package types

import "sync"

type cacheGuard struct {
	once sync.Once
	cond *sync.Cond
	mu   sync.RWMutex
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
