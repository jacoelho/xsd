package compiler

import (
	"maps"
	"slices"
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

func TestCollectProhibitedAttributesDeterministic(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:test"

	attrA := &types.AttributeDecl{
		Name:            types.QName{Local: "a"},
		Use:             types.Prohibited,
		Form:            types.FormUnqualified,
		SourceNamespace: schema.TargetNamespace,
	}
	attrB := &types.AttributeDecl{
		Name:            types.QName{Local: "b"},
		Use:             types.Prohibited,
		Form:            types.FormUnqualified,
		SourceNamespace: schema.TargetNamespace,
	}
	agQName := types.QName{Namespace: schema.TargetNamespace, Local: "ag"}
	schema.AttributeGroups[agQName] = &types.AttributeGroup{
		Name:            agQName,
		SourceNamespace: schema.TargetNamespace,
		Attributes:      []*types.AttributeDecl{attrB},
	}

	ctQName := types.QName{Namespace: schema.TargetNamespace, Local: "ct"}
	ct := types.NewComplexType(ctQName, schema.TargetNamespace)
	ct.SetAttributes([]*types.AttributeDecl{attrA})
	ct.AttrGroups = []types.QName{agQName}
	schema.TypeDefs[ctQName] = ct

	compiledSchema, err := NewCompiler(schema).Compile()
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	compiledType := compiledSchema.Types[ctQName]
	if compiledType == nil {
		t.Fatalf("compiled type %s not found", ctQName)
	}
	want := []types.QName{
		{Namespace: "", Local: "a"},
		{Namespace: "", Local: "b"},
	}
	if !slices.Equal(compiledType.ProhibitedAttributes, want) {
		t.Fatalf("prohibited attributes = %v, want %v", compiledType.ProhibitedAttributes, want)
	}
}
