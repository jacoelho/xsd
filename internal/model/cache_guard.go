package model

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

func (s *SimpleType) guard() *cacheGuard {
	if s == nil {
		return nil
	}
	s.cacheGuard.ensure()
	return &s.cacheGuard
}
