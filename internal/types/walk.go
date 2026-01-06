package types

// ParticleHandlers contains optional handler functions for particle traversal.
// Nil handlers are skipped.
type ParticleHandlers struct {
	OnElement  func(*ElementDecl) error
	OnGroup    func(*ModelGroup) error
	OnWildcard func(*AnyElement) error
	OnGroupRef func(*GroupRef) error
}

// WalkParticles recursively visits all particles, calling handlers for each type.
func WalkParticles(particles []Particle, h ParticleHandlers) error {
	for _, p := range particles {
		switch v := p.(type) {
		case *ElementDecl:
			if h.OnElement != nil {
				if err := h.OnElement(v); err != nil {
					return err
				}
			}

		case *ModelGroup:
			if h.OnGroup != nil {
				if err := h.OnGroup(v); err != nil {
					return err
				}
			}
			// Recurse into children
			if err := WalkParticles(v.Particles, h); err != nil {
				return err
			}

		case *AnyElement:
			if h.OnWildcard != nil {
				if err := h.OnWildcard(v); err != nil {
					return err
				}
			}

		case *GroupRef:
			if h.OnGroupRef != nil {
				if err := h.OnGroupRef(v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
