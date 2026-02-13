package complextypeplan

import (
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
)

func TestBuildAndLookup(t *testing.T) {
	ct := &model.ComplexType{QName: model.QName{Namespace: "urn:test", Local: "CT"}}
	attr := &model.AttributeDecl{Name: model.QName{Namespace: "urn:test", Local: "a"}}
	particle := &model.AnyElement{Namespace: model.NSCAny}
	textType := &model.SimpleType{QName: model.QName{Namespace: model.XSDNamespace, Local: "string"}}
	registry := &analysis.Registry{
		TypeOrder: []analysis.TypeEntry{
			{
				Type:  ct,
				QName: ct.QName,
			},
		},
	}

	plan, err := Build(registry, ComputeFuncs{
		AttributeUses: func(got *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, error) {
			if got != ct {
				t.Fatalf("attribute callback type = %p, want %p", got, ct)
			}
			return []*model.AttributeDecl{attr}, &model.AnyAttribute{Namespace: model.NSCAny}, nil
		},
		ContentParticle: func(got *model.ComplexType) model.Particle {
			if got != ct {
				t.Fatalf("content callback type = %p, want %p", got, ct)
			}
			return particle
		},
		SimpleContentType: func(got *model.ComplexType) (model.Type, error) {
			if got != ct {
				t.Fatalf("text callback type = %p, want %p", got, ct)
			}
			return textType, nil
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	attrs, wildcard, ok := plan.AttributeUses(ct)
	if !ok {
		t.Fatal("AttributeUses() missing entry")
	}
	if len(attrs) != 1 || attrs[0] != attr {
		t.Fatalf("attrs = %#v, want [%p]", attrs, attr)
	}
	if wildcard == nil || wildcard.Namespace != model.NSCAny {
		t.Fatalf("wildcard = %#v, want NSCAny", wildcard)
	}

	content, ok := plan.Content(ct)
	if !ok || content != particle {
		t.Fatalf("Content() = %v, %v; want %v, true", content, ok, particle)
	}

	gotType, ok := plan.SimpleContentType(ct)
	if !ok || gotType != textType {
		t.Fatalf("SimpleContentType() = %v, %v; want %v, true", gotType, ok, textType)
	}
}

func TestBuildNilRegistry(t *testing.T) {
	if _, err := Build(nil, ComputeFuncs{}); err == nil {
		t.Fatal("Build(nil, ...) expected error")
	}
}
