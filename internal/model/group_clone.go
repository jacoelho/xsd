package model

// CloneModelGroupTree clones model-group structure recursively.
// Non-model-group particles are reused by pointer.
func CloneModelGroupTree(group *ModelGroup) *ModelGroup {
	if group == nil {
		return nil
	}
	clone := *group
	if len(group.Particles) > 0 {
		clone.Particles = make([]Particle, len(group.Particles))
		for i, particle := range group.Particles {
			if nested, ok := particle.(*ModelGroup); ok {
				clone.Particles[i] = CloneModelGroupTree(nested)
				continue
			}
			clone.Particles[i] = particle
		}
	}
	return &clone
}
