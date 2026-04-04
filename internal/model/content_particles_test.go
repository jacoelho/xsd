package model

import "testing"

func TestCollectFromParticlesWithVisited(t *testing.T) {
	root := &ModelGroup{
		Particles: []Particle{
			&ElementDecl{Name: QName{Local: "a"}},
			&ModelGroup{
				Particles: []Particle{
					&AnyElement{},
					&ElementDecl{Name: QName{Local: "b"}},
				},
			},
		},
	}

	got := CollectFromParticlesWithVisited([]Particle{root}, nil, func(p Particle) (string, bool) {
		elem, ok := p.(*ElementDecl)
		if !ok {
			return "", false
		}
		return elem.Name.Local, true
	})

	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("collect from particle = %v, want [a b]", got)
	}
}

func TestCollectFromContent(t *testing.T) {
	content := &ComplexContent{
		Extension: &Extension{
			Particle: &ElementDecl{Name: QName{Local: "ext"}},
		},
		Restriction: &Restriction{
			Particle: &ElementDecl{Name: QName{Local: "restr"}},
		},
	}

	got := CollectFromContent(content, func(p Particle) (string, bool) {
		elem, ok := p.(*ElementDecl)
		if !ok {
			return "", false
		}
		return elem.Name.Local, true
	})

	if len(got) != 2 || got[0] != "ext" || got[1] != "restr" {
		t.Fatalf("collect from content = %v, want [ext restr]", got)
	}
}

func TestCollectFromContentHandlesCycles(t *testing.T) {
	g1 := &ModelGroup{}
	g2 := &ModelGroup{}
	g1.Particles = []Particle{
		&ElementDecl{Name: QName{Local: "a"}},
		g2,
	}
	g2.Particles = []Particle{
		&ElementDecl{Name: QName{Local: "b"}},
		g1,
	}

	content := &ComplexContent{
		Extension: &Extension{Particle: g1},
	}
	got := CollectFromContent(content, func(p Particle) (string, bool) {
		elem, ok := p.(*ElementDecl)
		if !ok {
			return "", false
		}
		return elem.Name.Local, true
	})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("collect from cyclic content = %v, want [a b]", got)
	}
}
