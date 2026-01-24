package compiler

import (
	"maps"
	"testing"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestAddCompiledAttributeDoesNotMutateReference(t *testing.T) {
	qname := types.QName{Namespace: "urn:attrs", Local: "code"}
	globalAttr := &types.AttributeDecl{
		Name:           qname,
		HasDefault:     true,
		Default:        "p:val",
		DefaultContext: map[string]string{"p": "urn:one"},
	}
	schema := &parser.Schema{
		TargetNamespace: "urn:attrs",
		AttributeDecls:  map[types.QName]*types.AttributeDecl{qname: globalAttr},
	}
	compiler := NewCompiler(schema)

	refAttr := &types.AttributeDecl{
		Name:        qname,
		IsReference: true,
	}
	attrMap := make(map[types.QName]*grammar.CompiledAttribute)

	if err := compiler.addCompiledAttribute(refAttr, attrMap); err != nil {
		t.Fatalf("addCompiledAttribute error = %v", err)
	}

	if refAttr.DefaultContext != nil {
		t.Fatalf("expected reference attribute DefaultContext to remain nil")
	}

	compiled := attrMap[qname]
	if compiled == nil {
		t.Fatalf("expected compiled attribute")
	}
	if compiled.Original == nil {
		t.Fatalf("expected compiled attribute to store original declaration")
	}
	if compiled.Original.DefaultContext == nil {
		t.Fatalf("expected compiled default context to be populated")
	}
	if !maps.Equal(compiled.Original.DefaultContext, globalAttr.DefaultContext) {
		t.Fatalf("expected compiled default context to match global context")
	}
}
