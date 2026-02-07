package traversal

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestCollectFromParticle(t *testing.T) {
	root := &types.ModelGroup{
		Particles: []types.Particle{
			&types.ElementDecl{Name: types.QName{Local: "a"}},
			&types.ModelGroup{
				Particles: []types.Particle{
					&types.AnyElement{},
					&types.ElementDecl{Name: types.QName{Local: "b"}},
				},
			},
		},
	}

	got := CollectFromParticle(root, func(p types.Particle) (string, bool) {
		elem, ok := p.(*types.ElementDecl)
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
	content := &types.ComplexContent{
		Extension: &types.Extension{
			Particle: &types.ElementDecl{Name: types.QName{Local: "ext"}},
		},
		Restriction: &types.Restriction{
			Particle: &types.ElementDecl{Name: types.QName{Local: "restr"}},
		},
	}

	got := CollectFromContent(content, func(p types.Particle) (string, bool) {
		elem, ok := p.(*types.ElementDecl)
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
	g1 := &types.ModelGroup{}
	g2 := &types.ModelGroup{}

	g1.Particles = []types.Particle{
		&types.ElementDecl{Name: types.QName{Local: "a"}, IsReference: true},
		g2,
	}
	g2.Particles = []types.Particle{
		&types.ElementDecl{Name: types.QName{Local: "b"}, IsReference: true},
		g1,
	}

	got := CollectFromParticlesWithVisited([]types.Particle{g1}, nil, func(p types.Particle) (string, bool) {
		elem, ok := p.(*types.ElementDecl)
		if !ok || !elem.IsReference {
			return "", false
		}
		return elem.Name.Local, true
	})

	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("collect with visited = %v, want [a b]", got)
	}
}
