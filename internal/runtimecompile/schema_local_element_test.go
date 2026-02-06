package runtimecompile

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestBuildSchema_LocalElementShadowsGlobalByQName(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:local"
           xmlns:tns="urn:local"
           elementFormDefault="qualified">
  <xs:element name="exterior" type="xs:string"/>
  <xs:complexType name="LocalT">
    <xs:sequence>
      <xs:element name="inner" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="ContainerType">
    <xs:sequence>
      <xs:element name="exterior" type="tns:LocalT"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="container" type="tns:ContainerType"/>
</xs:schema>`

	schema := mustResolveSchema(t, schemaXML)
	rt, err := BuildSchema(schema, BuildConfig{})
	if err != nil {
		t.Fatalf("BuildSchema error = %v", err)
	}

	nsID := rt.Namespaces.Lookup([]byte("urn:local"))
	if nsID == 0 {
		t.Fatalf("namespace not found")
	}
	symContainer := rt.Symbols.Lookup(nsID, []byte("container"))
	symExterior := rt.Symbols.Lookup(nsID, []byte("exterior"))
	symLocalT := rt.Symbols.Lookup(nsID, []byte("LocalT"))
	if symContainer == 0 || symExterior == 0 || symLocalT == 0 {
		t.Fatalf("symbols missing (container=%d exterior=%d LocalT=%d)", symContainer, symExterior, symLocalT)
	}

	var containerElem runtimeElementRef
	for i := 1; i < len(rt.Elements); i++ {
		if rt.Elements[i].Name == symContainer {
			containerElem = runtimeElementRef{id: uint32(i), elem: rt.Elements[i]}
			break
		}
	}
	if containerElem.id == 0 {
		t.Fatalf("container element not found")
	}
	containerType := rt.Types[containerElem.elem.Type]
	if containerType.Kind != runtime.TypeComplex {
		t.Fatalf("container type kind = %d, want complex", containerType.Kind)
	}
	ct := rt.ComplexTypes[containerType.Complex.ID]

	var matcher runtime.PosMatcher
	found := false
	switch ct.Model.Kind {
	case runtime.ModelDFA:
		model := rt.Models.DFA[ct.Model.ID]
		for _, tr := range model.Transitions {
			if tr.Sym == symExterior {
				matcher = runtime.PosMatcher{Kind: runtime.PosExact, Sym: tr.Sym, Elem: tr.Elem}
				found = true
				break
			}
		}
	case runtime.ModelNFA:
		model := rt.Models.NFA[ct.Model.ID]
		for _, m := range model.Matchers {
			if m.Kind == runtime.PosExact && m.Sym == symExterior {
				matcher = m
				found = true
				break
			}
		}
	default:
		t.Fatalf("unexpected model kind %d", ct.Model.Kind)
	}
	if !found {
		t.Fatalf("no matcher for local exterior")
	}

	if int(matcher.Elem) >= len(rt.Elements) {
		t.Fatalf("matcher elem %d out of range", matcher.Elem)
	}
	matchedElem := rt.Elements[matcher.Elem]
	matchedType := rt.Types[matchedElem.Type]
	if matchedType.Name != symLocalT {
		t.Fatalf("matcher element type = %d, want LocalT symbol %d", matchedType.Name, symLocalT)
	}
}

type runtimeElementRef struct {
	id   uint32
	elem runtime.Element
}
