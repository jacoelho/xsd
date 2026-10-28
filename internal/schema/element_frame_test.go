package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestElementFrameUsesEffectiveTypeAndDeclarationConstraint(t *testing.T) {
	t.Parallel()

	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatalf("value.Builder.Seal() error = %v", err)
	}
	stringID := value.BuiltinType(value.PrimitiveString)
	integerID, ok := value.BuiltinTypeID("integer")
	if !ok {
		t.Fatal("missing builtin integer type")
	}
	modelID := ContentModelID(1)
	rt := &Schema{program: schemaProgram{
		Value:                 program,
		SimpleTypeUnavailable: make([]bool, value.BuiltinTypeCount),
		ComplexTypes: []complexTypeRead{
			{},
			{
				contentModel: modelID,
				textType:     stringID,
				contentKind:  ContentSimple,
			},
		},
		CompiledModels: testCompiledModelReads([]CompiledModel{
			{},
			{Start: 7, AllBitLen: 2},
		}),
		Elements: newElementReadTable([]ElementDecl{
			{Type: SimpleRef(stringID)},
			{Type: SimpleRef(integerID), Fixed: &ValueConstraint{}},
		}),
	}}

	read, ok := rt.ElementFrame(ComplexRef(1), 1)
	if !ok {
		t.Fatal("ElementFrame() failed for valid effective type and declaration")
	}
	if read.SimpleContent != stringID {
		t.Fatalf("SimpleContent = %d, want %d from effective type", read.SimpleContent, stringID)
	}
	if read.TextContent.AllowsMixedContent() || !read.TextContent.HasFixedElementValue() || !read.TextContent.HasValueConstraint() {
		t.Fatalf("TextContent = %+v, want simple content and fixed constraint", read.TextContent)
	}
	state := read.Content.ContentState()
	if state.model != modelID || state.state != 7 || read.Content.AllBitLen() != 2 {
		t.Fatalf("Content = %+v, want model %d start 7 all-bit length 2", read.Content, modelID)
	}

	// The declaration's type is deliberately simple while the effective type
	// is complex. The frame must use the selected effective type and retain the
	// declaration only for constraints.
	complexRead, ok := rt.ElementFrame(ComplexRef(0), 0)
	if !ok {
		t.Fatal("ElementFrame() failed for valid complex type without simple content")
	}
	if complexRead.SimpleContent != NoSimpleType {
		t.Fatalf("complex type without simple content = %d, want NoSimpleType", complexRead.SimpleContent)
	}

	if _, ok := rt.ElementFrame(TypeID{}, 1); ok {
		t.Fatal("ElementFrame() accepted an untyped runtime type")
	}
	if _, ok := rt.ElementFrame(ComplexRef(1), 2); ok {
		t.Fatal("ElementFrame() accepted an invalid declaration ID")
	}
}

func TestSchemaReadsRejectInvalidSimpleContent(t *testing.T) {
	program, sealErr := value.NewBuilder(value.BuilderOptions{}).Seal()
	if sealErr != nil {
		t.Fatal(sealErr)
	}
	stringID := value.BuiltinType(value.PrimitiveString)
	rt := &Schema{program: schemaProgram{
		Value:                 program,
		SimpleTypeUnavailable: make([]bool, value.BuiltinTypeCount),
		ComplexTypes: []complexTypeRead{
			{contentModel: NoContentModel, textType: NoSimpleType, contentKind: ContentSimple},
			{contentModel: NoContentModel, textType: stringID, contentKind: ContentSimple},
			{contentModel: NoContentModel, textType: stringID, contentKind: ContentEmpty},
		},
	}}
	for _, id := range []ComplexTypeID{0} {
		if info, valid := rt.TypeInfo(ComplexRef(id)); valid {
			t.Fatalf("TypeInfo(%d) accepted present invalid simple content: %+v", id, info)
		}
		if frame, valid := rt.ElementFrame(ComplexRef(id), NoElement); valid {
			t.Fatalf("ElementFrame(%d) accepted present invalid simple content: %+v", id, frame)
		}
	}
	if info, valid := rt.TypeInfo(ComplexRef(1)); !valid || info.Unavailable {
		t.Fatalf("TypeInfo(valid simple content) = %+v, %v", info, valid)
	}
	if frame, valid := rt.ElementFrame(ComplexRef(1), NoElement); !valid || frame.SimpleContent != stringID {
		t.Fatalf("ElementFrame(valid simple content) = %+v, %v; want string type", frame, valid)
	}
	if _, valid := rt.TypeInfo(ComplexRef(2)); !valid {
		t.Fatal("TypeInfo(absent simple content with stray text ID) failed")
	}
	if frame, valid := rt.ElementFrame(ComplexRef(2), NoElement); !valid || frame.SimpleContent != NoSimpleType {
		t.Fatalf("ElementFrame(absent simple content) = %+v, %v; want NoSimpleType", frame, valid)
	}
}
