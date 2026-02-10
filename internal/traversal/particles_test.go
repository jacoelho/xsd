package traversal

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestCollectFromParticle(t *testing.T) {
	root := &model.ModelGroup{
		Particles: []model.Particle{
			&model.ElementDecl{Name: model.QName{Local: "a"}},
			&model.ModelGroup{
				Particles: []model.Particle{
					&model.AnyElement{},
					&model.ElementDecl{Name: model.QName{Local: "b"}},
				},
			},
		},
	}

	got := CollectFromParticlesWithVisited([]model.Particle{root}, nil, func(p model.Particle) (string, bool) {
		elem, ok := p.(*model.ElementDecl)
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
	content := &model.ComplexContent{
		Extension: &model.Extension{
			Particle: &model.ElementDecl{Name: model.QName{Local: "ext"}},
		},
		Restriction: &model.Restriction{
			Particle: &model.ElementDecl{Name: model.QName{Local: "restr"}},
		},
	}

	got := CollectFromContent(content, func(p model.Particle) (string, bool) {
		elem, ok := p.(*model.ElementDecl)
		if !ok {
			return "", false
		}
		return elem.Name.Local, true
	})

	if len(got) != 2 || got[0] != "ext" || got[1] != "restr" {
		t.Fatalf("collect from content = %v, want [ext restr]", got)
	}
}

func TestCollectFromParticlesWithVisited(t *testing.T) {
	g1 := &model.ModelGroup{}
	g2 := &model.ModelGroup{}

	g1.Particles = []model.Particle{
		&model.ElementDecl{Name: model.QName{Local: "a"}, IsReference: true},
		g2,
	}
	g2.Particles = []model.Particle{
		&model.ElementDecl{Name: model.QName{Local: "b"}, IsReference: true},
		g1,
	}

	got := CollectFromParticlesWithVisited([]model.Particle{g1}, nil, func(p model.Particle) (string, bool) {
		elem, ok := p.(*model.ElementDecl)
		if !ok || !elem.IsReference {
			return "", false
		}
		return elem.Name.Local, true
	})

	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("collect with visited = %v, want [a b]", got)
	}
}
