package typewalk

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestWalkVisitsChainOnce(t *testing.T) {
	root := &model.SimpleType{QName: model.QName{Namespace: "urn:test", Local: "Root"}}
	mid := &model.SimpleType{
		QName:        model.QName{Namespace: "urn:test", Local: "Mid"},
		ResolvedBase: root,
		Restriction:  &model.Restriction{},
	}
	leaf := &model.SimpleType{
		QName:        model.QName{Namespace: "urn:test", Local: "Leaf"},
		ResolvedBase: mid,
		Restriction:  &model.Restriction{},
	}

	var order []model.QName
	Walk(leaf, func(current model.Type) model.Type {
		st, _ := model.AsSimpleType(current)
		if st == nil {
			return nil
		}
		return st.ResolvedBase
	}, func(current model.Type) bool {
		order = append(order, current.Name())
		return true
	})

	if len(order) != 3 {
		t.Fatalf("visited %d types, want 3", len(order))
	}
	if order[0] != leaf.QName || order[1] != mid.QName || order[2] != root.QName {
		t.Fatalf("visit order = %v, want [%v %v %v]", order, leaf.QName, mid.QName, root.QName)
	}
}

func TestWalkStopsOnCycle(t *testing.T) {
	a := &model.SimpleType{QName: model.QName{Namespace: "urn:test", Local: "A"}}
	b := &model.SimpleType{QName: model.QName{Namespace: "urn:test", Local: "B"}}
	a.ResolvedBase = b
	b.ResolvedBase = a

	count := 0
	Walk(a, func(current model.Type) model.Type {
		st, _ := model.AsSimpleType(current)
		if st == nil {
			return nil
		}
		return st.ResolvedBase
	}, func(model.Type) bool {
		count++
		return true
	})

	if count != 2 {
		t.Fatalf("visited %d types, want 2", count)
	}
}
