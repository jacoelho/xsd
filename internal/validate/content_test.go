package validate

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

type contentRuntimeStub struct {
	compiledContentModel func(runtime.ContentModelID) (runtime.CompiledModel, bool)
	textContent          characterDataContentStub
	childContent         map[runtime.TypeID]runtime.ChildContentInfo
	contentModels        map[runtime.TypeID]runtime.ContentModelID
	elementTypes         map[runtime.ElementID]runtime.TypeID
	models               map[runtime.ContentModelID]runtime.CompiledModel
	globalElements       map[runtime.QName]runtime.ElementID
	elementNames         map[runtime.ElementID]runtime.QName
	wildcards            map[runtime.WildcardID]runtime.Wildcard
	substitutionLookup   map[runtime.ElementID]map[runtime.QName]runtime.ElementID
	anyType              runtime.TypeID
	textContentOK        bool
}

func (s contentRuntimeStub) AnyType() runtime.TypeID {
	return s.anyType
}

func (s contentRuntimeStub) ChildContent(id runtime.TypeID) (runtime.ChildContentInfo, bool) {
	info, ok := s.childContent[id]
	return info, ok
}

func (s contentRuntimeStub) ElementTextContent(runtime.TypeID, runtime.ElementID) (characterDataContentStub, bool) {
	return s.textContent, s.textContentOK
}

func (s contentRuntimeStub) ContentModelForType(id runtime.TypeID) runtime.ContentModelID {
	if s.contentModels == nil {
		return runtime.NoContentModel
	}
	return s.contentModels[id]
}

func (s contentRuntimeStub) DeclaredElementType(id runtime.ElementID) (runtime.TypeID, bool) {
	typ, ok := s.elementTypes[id]
	return typ, ok
}

func (s contentRuntimeStub) CompiledContentModelView(id runtime.ContentModelID) (runtime.CompiledModelView, bool) {
	if s.compiledContentModel != nil {
		model, ok := s.compiledContentModel(id)
		if !ok {
			return runtime.CompiledModelView{}, false
		}
		return runtime.NewCompiledModelView(&model), true
	}
	model, ok := s.models[id]
	if !ok {
		return runtime.CompiledModelView{}, false
	}
	return runtime.NewCompiledModelView(&model), true
}

func (s contentRuntimeStub) GlobalElement(name runtime.QName) (runtime.ElementID, bool) {
	id, ok := s.globalElements[name]
	return id, ok
}

func (s contentRuntimeStub) ElementName(id runtime.ElementID) (runtime.QName, bool) {
	name, ok := s.elementNames[id]
	return name, ok
}

func (s contentRuntimeStub) WildcardView(id runtime.WildcardID) (runtime.WildcardView, bool) {
	w, ok := s.wildcards[id]
	if !ok {
		return runtime.WildcardView{}, false
	}
	return runtime.NewWildcardView(nil, &w), true
}

func (s contentRuntimeStub) SubstitutionMemberByName(id runtime.ElementID, name runtime.QName) (runtime.ElementID, bool) {
	members := s.substitutionLookup[id]
	if members == nil {
		return runtime.NoElement, false
	}
	member, ok := members[name]
	return member, ok
}

type characterDataContentStub struct {
	simple  bool
	complex bool
	mixed   bool
	fixed   bool
}

func (s characterDataContentStub) HasSimpleContent() bool {
	return s.simple
}

func (s characterDataContentStub) IsComplexType() bool {
	return s.complex
}

func (s characterDataContentStub) AllowsMixedContent() bool {
	return s.mixed
}

func (s characterDataContentStub) HasFixedElementValue() bool {
	return s.fixed
}

func schemaContentRuntimeForTest(
	model runtime.CompiledModel,
	elementNames []runtime.QName,
	elementTypes []runtime.TypeID,
	wildcards []runtime.Wildcard,
) *runtime.Schema {
	elementStartInfos := make([]runtime.ElementStartInfo, len(elementTypes))
	complexCount := 1
	for i, typ := range elementTypes {
		elementStartInfos[i] = runtime.NewElementStartInfo(runtime.ElementStartInfoShape{Type: typ})
		if id, ok := typ.Complex(); ok && int(id) >= complexCount {
			complexCount = int(id) + 1
		}
	}
	contentModels := make([]runtime.ContentModelID, complexCount)
	for i := range contentModels {
		contentModels[i] = runtime.NoContentModel
	}
	contentModels[0] = 0
	childContent := make([]runtime.ElementChildContent, complexCount)
	for i := range childContent {
		childContent[i] = runtime.NewElementChildContent(runtime.ElementChildContentShape{Complex: true})
	}
	return &runtime.Schema{
		ComplexContentModelIDs:   contentModels,
		ComplexChildContentReads: childContent,
		CompiledModelViews:       runtime.NewCompiledModelViews([]runtime.CompiledModel{model}),
		ElementNames:             elementNames,
		ElementStartInfos:        elementStartInfos,
		WildcardReads:            runtime.NewWildcardViews(nil, wildcards),
	}
}

func TestChildStartAdvancesContentAndReturnsDeclaredType(t *testing.T) {
	t.Parallel()

	parent := runtime.ComplexRef(1)
	childType := runtime.ComplexRef(2)
	child := runtime.ElementID(3)
	model := runtime.ContentModelID(4)
	childName := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	compiled := runtime.CompiledModel{
		Kind:  runtime.CompiledModelDFA,
		Start: 7,
		Rows:  make([]runtime.CompiledModelRow, 9),
	}
	compiled.Rows[7] = runtime.CompiledModelRow{
		Edges: []runtime.CompiledModelEdge{{
			Particle: runtime.ElementParticle(child, runtime.Occurrence{Min: 1, Max: 1}),
			To:       8,
		}},
	}
	compiled.Rows[8] = runtime.CompiledModelRow{Accept: true}
	rt := contentRuntimeStub{
		anyType:       runtime.ComplexRef(100),
		childContent:  map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true}},
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
		elementTypes:  map[runtime.ElementID]runtime.TypeID{child: childType},
		models:        map[runtime.ContentModelID]runtime.CompiledModel{model: compiled},
		elementNames:  map[runtime.ElementID]runtime.QName{child: childName},
	}
	parentContent := runtime.ContentFrameForType(rt, parent).ContentState()

	got, err := ChildStart(rt, ChildInput{
		Context:       StartContext{Path: "/root", Line: 2, Column: 3},
		Scratch:       runtime.NewContentScratch(make([]uint64, 1), 0, 1),
		Name:          runtime.RuntimeName{Known: true, Name: childName, Local: "child"},
		ParentContent: parentContent,
		ParentType:    parent,
	})
	if err != nil {
		t.Fatalf("ChildStart() error = %v", err)
	}
	if got.Element != child || got.Type != childType || got.Skip || !got.Content.HasModel() {
		t.Fatalf("ChildStart() = %+v", got)
	}
	if err := ContentComplete(rt, CompleteInput{Type: parent, Content: got.Content}); err != nil {
		t.Fatalf("ContentComplete(updated content) error = %v", err)
	}
}

func TestChildStartStrictWildcardRecoveryPreservesAdvancedContent(t *testing.T) {
	t.Parallel()

	parent := runtime.ComplexRef(1)
	childType := runtime.ComplexRef(2)
	after := runtime.ElementID(3)
	model := runtime.ContentModelID(4)
	wildcard := runtime.WildcardID(5)
	afterName := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	compiled := runtime.CompiledModel{
		Kind: runtime.CompiledModelDFA,
		Rows: []runtime.CompiledModelRow{
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.WildcardParticle(wildcard, runtime.Occurrence{Min: 1, Max: 1}),
					To:       1,
				}},
			},
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.ElementParticle(after, runtime.Occurrence{Min: 1, Max: 1}),
					To:       2,
				}},
			},
			{Accept: true},
		},
	}
	rt := contentRuntimeStub{
		anyType:       runtime.ComplexRef(100),
		childContent:  map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true}},
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
		elementTypes:  map[runtime.ElementID]runtime.TypeID{after: childType},
		models:        map[runtime.ContentModelID]runtime.CompiledModel{model: compiled},
		elementNames:  map[runtime.ElementID]runtime.QName{after: afterName},
		wildcards:     map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}},
	}
	parentContent := runtime.ContentFrameForType(rt, parent).ContentState()

	first, err := ChildStart(rt, ChildInput{
		Context:       StartContext{Path: "/root", Line: 2, Column: 3},
		Name:          runtime.RuntimeName{Known: true, Name: runtime.QName{Local: 9}, Local: "unknown"},
		ParentContent: parentContent,
		ParentType:    parent,
	})
	expectXSDCode(t, err, xsderrors.CodeValidationElement)
	if !first.ContentAdvanced || !first.Skip || !first.Recover {
		t.Fatalf("strict wildcard result = %+v, want advanced recoverable skipped child", first)
	}

	second, err := ChildStart(rt, ChildInput{
		Context:       StartContext{Path: "/root", Line: 3, Column: 3},
		Name:          runtime.RuntimeName{Known: true, Name: afterName, Local: "after"},
		ParentContent: first.Content,
		ParentType:    parent,
	})
	if err != nil {
		t.Fatalf("ChildStart(after) error = %v", err)
	}
	if second.Element != after || second.Type != childType || second.Skip {
		t.Fatalf("ChildStart(after) = %+v", second)
	}
	if err := ContentComplete(rt, CompleteInput{Type: parent, Content: second.Content}); err != nil {
		t.Fatalf("ContentComplete(updated content) error = %v", err)
	}
}

func TestAcceptChildSchemaStrictWildcardRecoveryAdvancesContent(t *testing.T) {
	t.Parallel()

	parentType := runtime.ComplexRef(0)
	afterType := runtime.ComplexRef(2)
	afterElement := runtime.ElementID(0)
	afterName := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	wildcard := runtime.WildcardID(0)
	model := runtime.CompiledModel{
		Kind: runtime.CompiledModelDFA,
		Rows: []runtime.CompiledModelRow{
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.WildcardParticle(wildcard, runtime.Occurrence{Min: 1, Max: 1}),
					To:       1,
				}},
			},
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.ElementParticle(afterElement, runtime.Occurrence{Min: 1, Max: 1}),
					To:       2,
				}},
			},
			{Accept: true},
		},
	}
	rt := schemaContentRuntimeForTest(
		model,
		[]runtime.QName{afterName},
		[]runtime.TypeID{afterType},
		[]runtime.Wildcard{{Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}},
	)
	parent := frame{
		Type:    parentType,
		Content: rt.ContentFrameForPublishedSchema(parentType).ContentState(),
		Child:   runtime.ChildContentInfo{Complex: true},
		ChildOK: true,
	}
	s := session{schema: rt}

	first, err := s.acceptChild(&parent, runtime.RuntimeName{
		Known: true,
		Name:  runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 9},
		Local: "unknown",
	}, false, 2, 3)
	expectXSDCode(t, err, xsderrors.CodeValidationElement)
	if !first.recover || !first.skip {
		t.Fatalf("acceptChild(strict wildcard) = %+v, want recoverable skipped child", first)
	}

	second, err := s.acceptChild(&parent, runtime.RuntimeName{
		Known: true,
		Name:  afterName,
		Local: "after",
	}, false, 3, 3)
	if err != nil {
		t.Fatalf("acceptChild(after) error = %v", err)
	}
	if second.element != afterElement || second.typ != afterType || second.skip {
		t.Fatalf("acceptChild(after) = %+v", second)
	}
	if err := s.completeFrame(&parent, 4, 3); err != nil {
		t.Fatalf("completeFrame(updated content) error = %v", err)
	}
}

func TestAcceptChildSchemaParentSkipPreservesContent(t *testing.T) {
	t.Parallel()

	parentType := runtime.ComplexRef(0)
	childElement := runtime.ElementID(0)
	childName := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	model := runtime.CompiledModel{
		Kind: runtime.CompiledModelDFA,
		Rows: []runtime.CompiledModelRow{
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.ElementParticle(childElement, runtime.Occurrence{Min: 1, Max: 1}),
					To:       1,
				}},
			},
			{Accept: true},
		},
	}
	rt := schemaContentRuntimeForTest(model, []runtime.QName{childName}, []runtime.TypeID{runtime.ComplexRef(1)}, nil)
	before := rt.ContentFrameForPublishedSchema(parentType).ContentState()
	parent := frame{
		Type:    parentType,
		Content: before,
		Child:   runtime.ChildContentInfo{Complex: true},
		ChildOK: true,
		Skip:    true,
	}
	s := session{schema: rt}

	accepted, err := s.acceptChild(&parent, runtime.RuntimeName{
		Known: true,
		Name:  childName,
		Local: "child",
	}, false, 2, 3)
	if err != nil {
		t.Fatalf("acceptChild(skipped parent) error = %v", err)
	}
	if !accepted.skip || accepted.element != runtime.NoElement || accepted.typ != rt.AnyType() {
		t.Fatalf("acceptChild(skipped parent) = %+v, want skipped anyType child", accepted)
	}
	if parent.Content != before {
		t.Fatalf("parent content changed for skipped parent")
	}
}

func TestAcceptChildSchemaLocationStrictWildcardIsUnsupported(t *testing.T) {
	t.Parallel()

	parentType := runtime.ComplexRef(0)
	wildcard := runtime.WildcardID(0)
	model := runtime.CompiledModel{
		Kind: runtime.CompiledModelDFA,
		Rows: []runtime.CompiledModelRow{
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.WildcardParticle(wildcard, runtime.Occurrence{Min: 1, Max: 1}),
					To:       1,
				}},
			},
			{Accept: true},
		},
	}
	rt := schemaContentRuntimeForTest(
		model,
		nil,
		nil,
		[]runtime.Wildcard{{Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}},
	)
	before := rt.ContentFrameForPublishedSchema(parentType).ContentState()
	parent := frame{
		Type:    parentType,
		Content: before,
		Child:   runtime.ChildContentInfo{Complex: true},
		ChildOK: true,
	}
	s := session{schema: rt}
	s.doc.schemaLocationHints.namespaces = map[string]bool{"urn:t": true}

	got, err := s.acceptChild(&parent, runtime.RuntimeName{NS: "urn:t", Local: "child"}, false, 2, 3)
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
	if got.recover || got.skip {
		t.Fatalf("acceptChild(schemaLocation strict wildcard) = %+v, want non-recoverable unsupported schemaLocation", got)
	}
	if parent.Content != before {
		t.Fatalf("parent content changed for unsupported schemaLocation")
	}
}

func TestAcceptChildSchemaUsesElementStartInfos(t *testing.T) {
	t.Parallel()

	parentType := runtime.ComplexRef(0)
	childType := runtime.ComplexRef(1)
	childElement := runtime.ElementID(0)
	childName := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	model := runtime.CompiledModel{
		Kind: runtime.CompiledModelDFA,
		Rows: []runtime.CompiledModelRow{
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.ElementParticle(childElement, runtime.Occurrence{Min: 1, Max: 1}),
					To:       1,
				}},
			},
			{Accept: true},
		},
	}
	rt := schemaContentRuntimeForTest(model, []runtime.QName{childName}, []runtime.TypeID{childType}, nil)
	parent := frame{
		Type:    parentType,
		Content: rt.ContentFrameForPublishedSchema(parentType).ContentState(),
		Child:   runtime.ChildContentInfo{Complex: true},
		ChildOK: true,
	}
	s := session{schema: rt}

	got, err := s.acceptChild(&parent, runtime.RuntimeName{Known: true, Name: childName, Local: "child"}, false, 2, 3)
	if err != nil {
		t.Fatalf("acceptChild(schema element) error = %v", err)
	}
	if got.element != childElement || got.typ != childType || got.skip {
		t.Fatalf("acceptChild(schema element) = %+v, want element %d type %v", got, childElement, childType)
	}
}

func TestChildStartInvalidCompiledContentIsInternalInvariant(t *testing.T) {
	t.Parallel()

	model := runtime.ContentModelID(4)
	parent := runtime.ComplexRef(1)
	child := runtime.ElementID(3)
	childName := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	tests := []struct {
		name  string
		index runtime.DFARowIndex
	}{
		{name: "linear"},
		{
			name: "indexed",
			index: runtime.DFARowIndex{
				NameToEdge: map[runtime.QName]uint32{childName: 0},
				Enabled:    true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			compiled := runtime.CompiledModel{
				Kind: runtime.CompiledModelDFA,
				Rows: []runtime.CompiledModelRow{
					{
						Index: tc.index,
						Edges: []runtime.CompiledModelEdge{{
							Particle: runtime.ElementParticle(child, runtime.Occurrence{Min: 1, Max: 1}),
							To:       1,
						}},
					},
					{Accept: true},
				},
			}
			rt := contentRuntimeStub{
				anyType:       runtime.ComplexRef(100),
				childContent:  map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true}},
				contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
				models:        map[runtime.ContentModelID]runtime.CompiledModel{model: compiled},
			}
			parentContent := runtime.ContentFrameForType(rt, parent).ContentState()
			_, err := ChildStart(rt, ChildInput{
				Context:       StartContext{Path: "/root", Line: 2, Column: 3},
				Name:          runtime.RuntimeName{Known: true, Name: childName, Local: "child"},
				ParentContent: parentContent,
				ParentType:    parent,
			})
			expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
		})
	}
}

func TestChildStartRejectsInvalidMatchedElementType(t *testing.T) {
	t.Parallel()

	parent := runtime.ComplexRef(1)
	child := runtime.ElementID(3)
	model := runtime.ContentModelID(4)
	childName := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	compiled := runtime.CompiledModel{
		Kind: runtime.CompiledModelDFA,
		Rows: []runtime.CompiledModelRow{
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.ElementParticle(child, runtime.Occurrence{Min: 1, Max: 1}),
					To:       1,
				}},
			},
			{Accept: true},
		},
	}
	rt := contentRuntimeStub{
		anyType:       runtime.ComplexRef(100),
		childContent:  map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true}},
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
		models:        map[runtime.ContentModelID]runtime.CompiledModel{model: compiled},
		elementNames:  map[runtime.ElementID]runtime.QName{child: childName},
	}
	parentContent := runtime.ContentFrameForType(rt, parent).ContentState()

	_, err := ChildStart(rt, ChildInput{
		Context:       StartContext{Path: "/root", Line: 2, Column: 3},
		Name:          runtime.RuntimeName{Known: true, Name: childName, Local: "child"},
		ParentContent: parentContent,
		ParentType:    parent,
	})
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
}

func TestChildStartRejectsInvalidParentContentMetadata(t *testing.T) {
	t.Parallel()

	_, err := ChildStart(contentRuntimeStub{
		anyType: runtime.ComplexRef(100),
	}, ChildInput{
		Context:    StartContext{Path: "/root", Line: 2, Column: 3},
		ParentType: runtime.ComplexRef(1),
	})
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
}

func TestValidateCharacterData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   CharacterDataInput
		want    CharacterDataResult
		wantErr xsderrors.Code
	}{
		{
			name: "CDATA outside root",
			input: CharacterDataInput{
				Data:  []byte("x"),
				CDATA: true,
			},
			wantErr: xsderrors.CodeValidationXML,
		},
		{
			name: "text outside root",
			input: CharacterDataInput{
				Data: []byte("x"),
			},
			wantErr: xsderrors.CodeValidationText,
		},
		{
			name: "whitespace outside root",
			input: CharacterDataInput{
				Data: []byte(" \n\t"),
			},
		},
		{
			name: "simple content appends text",
			input: CharacterDataInput{
				Data:       []byte("x"),
				Content:    characterDataContentStub{simple: true},
				HasElement: true,
			},
			want: CharacterDataResult{AppendText: true},
		},
		{
			name: "mixed fixed content appends and records text",
			input: CharacterDataInput{
				Data:       []byte("x"),
				Content:    characterDataContentStub{complex: true, mixed: true, fixed: true},
				HasElement: true,
			},
			want: CharacterDataResult{AppendText: true, HasText: true},
		},
		{
			name: "complex whitespace is allowed",
			input: CharacterDataInput{
				Data:       []byte(" \n\t"),
				Content:    characterDataContentStub{complex: true},
				HasElement: true,
			},
		},
		{
			name: "complex text is rejected",
			input: CharacterDataInput{
				Data:       []byte("x"),
				Content:    characterDataContentStub{complex: true},
				HasElement: true,
			},
			wantErr: xsderrors.CodeValidationText,
		},
		{
			name: "nilled text is rejected",
			input: CharacterDataInput{
				Data:       []byte(" "),
				Content:    characterDataContentStub{simple: true},
				HasElement: true,
				Nilled:     true,
			},
			wantErr: xsderrors.CodeValidationNil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.input.Context = StartContext{Path: "/root", Line: 2, Column: 3}
			got, err := ValidateCharacterData(tt.input)
			if tt.wantErr != "" {
				expectXSDCode(t, err, tt.wantErr)
				return
			}
			if err != nil {
				t.Fatalf("ValidateCharacterData() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ValidateCharacterData() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateElementCharacterDataUsesRuntimeContent(t *testing.T) {
	t.Parallel()

	got, err := ValidateElementCharacterData(contentRuntimeStub{
		textContent:   characterDataContentStub{simple: true},
		textContentOK: true,
	}, ElementCharacterDataInput{
		Data:    []byte("x"),
		Type:    runtime.ComplexRef(1),
		Element: 1,
		Context: StartContext{Path: "/root", Line: 2, Column: 3},
	})
	if err != nil {
		t.Fatalf("ValidateElementCharacterData() error = %v", err)
	}
	if got != (CharacterDataResult{AppendText: true}) {
		t.Fatalf("ValidateElementCharacterData() = %+v, want append text", got)
	}
}

func TestValidateElementCharacterDataRejectsMissingContentMetadata(t *testing.T) {
	t.Parallel()

	_, err := ValidateElementCharacterData(contentRuntimeStub{}, ElementCharacterDataInput{
		Data:    []byte("x"),
		Type:    runtime.ComplexRef(1),
		Element: 1,
		Context: StartContext{Path: "/root", Line: 2, Column: 3},
	})
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
}

func TestChildStartRejectsInvalidParentContent(t *testing.T) {
	t.Parallel()

	parent := runtime.ComplexRef(1)
	tests := []struct {
		name string
		in   ChildInput
		rt   contentRuntimeStub
		code xsderrors.Code
	}{
		{
			name: "nilled",
			in: ChildInput{
				ParentNilled: true,
			},
			code: xsderrors.CodeValidationNil,
		},
		{
			name: "simple type",
			in: ChildInput{
				ParentType: parent,
			},
			rt: contentRuntimeStub{
				childContent: map[runtime.TypeID]runtime.ChildContentInfo{parent: {}},
			},
			code: xsderrors.CodeValidationContent,
		},
		{
			name: "simple content",
			in: ChildInput{
				ParentType: parent,
			},
			rt: contentRuntimeStub{
				childContent: map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true, Simple: true}},
			},
			code: xsderrors.CodeValidationContent,
		},
		{
			name: "no model",
			in: ChildInput{
				Name:       runtime.RuntimeName{Local: "child"},
				ParentType: parent,
			},
			rt: contentRuntimeStub{
				childContent: map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true}},
			},
			code: xsderrors.CodeValidationElement,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.rt.anyType == (runtime.TypeID{}) {
				tc.rt.anyType = runtime.ComplexRef(100)
			}
			tc.in.Context = StartContext{Path: "/root", Line: 2, Column: 3}
			got, err := ChildStart(tc.rt, tc.in)
			expectXSDCode(t, err, tc.code)
			if got.Element != runtime.NoElement || got.Type != tc.rt.anyType || !got.Skip || !got.Recover {
				t.Fatalf("ChildStart() = %+v, want recoverable skipped anyType child", got)
			}
		})
	}
}

func TestChildStartPolicyClassifiesSharedValidationRules(t *testing.T) {
	t.Parallel()

	name := runtime.RuntimeName{Local: "child"}
	if got := childFramePolicy(true, true); !got.skip || got.issue.valid() {
		t.Fatalf("childFramePolicy(skip) = %+v, want skipped without validation issue", got)
	}
	if got := childFramePolicy(false, true); got.skip || got.issue.code != xsderrors.CodeValidationNil {
		t.Fatalf("childFramePolicy(nilled) = %+v, want nil validation issue", got)
	}

	parent := runtime.ComplexRef(1)
	model := runtime.ContentModelID(4)
	rt := contentRuntimeStub{
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
		models: map[runtime.ContentModelID]runtime.CompiledModel{
			model: {Kind: runtime.CompiledModelDFA, Rows: []runtime.CompiledModelRow{{Accept: true}}},
		},
	}
	content := runtime.ContentFrameForType(rt, parent).ContentState()
	tests := []struct {
		name    string
		child   runtime.ChildContentInfo
		content runtime.ContentState
		code    xsderrors.Code
	}{
		{name: "simple type", code: xsderrors.CodeValidationContent},
		{name: "simple content", child: runtime.ChildContentInfo{Complex: true, Simple: true}, code: xsderrors.CodeValidationContent},
		{name: "no model", child: runtime.ChildContentInfo{Complex: true}, code: xsderrors.CodeValidationElement},
		{name: "accepted", child: runtime.ChildContentInfo{Complex: true}, content: content},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := childContentPolicy(tc.child, tc.content, name)
			if got.code != tc.code {
				t.Fatalf("childContentPolicy() = %+v, want code %q", got, tc.code)
			}
		})
	}
}

func TestContentCompletionRequiredPolicy(t *testing.T) {
	t.Parallel()

	parent := runtime.ComplexRef(1)
	model := runtime.ContentModelID(4)
	rt := contentRuntimeStub{
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
		models: map[runtime.ContentModelID]runtime.CompiledModel{
			model: {Kind: runtime.CompiledModelDFA, Rows: []runtime.CompiledModelRow{{Accept: true}}},
		},
	}
	content := runtime.ContentFrameForType(rt, parent).ContentState()
	if !contentCompletionRequired(false, parent, content) {
		t.Fatal("contentCompletionRequired(complex model) = false, want true")
	}
	if contentCompletionRequired(true, parent, content) {
		t.Fatal("contentCompletionRequired(nilled) = true, want false")
	}
	if contentCompletionRequired(false, runtime.SimpleRef(1), content) {
		t.Fatal("contentCompletionRequired(simple type) = true, want false")
	}
	if contentCompletionRequired(false, parent, runtime.ContentState{}) {
		t.Fatal("contentCompletionRequired(no model) = true, want false")
	}
}

func TestValidateNilledContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   NilledContentInput
		wantErr bool
	}{
		{
			name: "nilled with text",
			input: NilledContentInput{
				Nilled:  true,
				HasText: true,
			},
			wantErr: true,
		},
		{
			name: "nilled with child",
			input: NilledContentInput{
				Nilled:   true,
				HasChild: true,
			},
			wantErr: true,
		},
		{
			name: "nilled empty",
			input: NilledContentInput{
				Nilled: true,
			},
		},
		{
			name: "not nilled with content",
			input: NilledContentInput{
				HasText:  true,
				HasChild: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.input.Context = StartContext{Path: "/root", Line: 2, Column: 3}
			err := ValidateNilledContent(tt.input)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ValidateNilledContent() error = %v", err)
				}
				return
			}
			expectXSDCode(t, err, xsderrors.CodeValidationNil)
		})
	}
}

func TestChildStartUnexpectedChildIsRecoverable(t *testing.T) {
	t.Parallel()

	parent := runtime.ComplexRef(1)
	model := runtime.ContentModelID(4)
	compiled := runtime.CompiledModel{
		Kind: runtime.CompiledModelDFA,
		Rows: []runtime.CompiledModelRow{{}},
	}
	rt := contentRuntimeStub{
		anyType:       runtime.ComplexRef(100),
		childContent:  map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true}},
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
		models:        map[runtime.ContentModelID]runtime.CompiledModel{model: compiled},
	}
	parentContent := runtime.ContentFrameForType(rt, parent).ContentState()
	got, err := ChildStart(rt, ChildInput{
		Context:       StartContext{Path: "/root", Line: 2, Column: 3},
		Name:          runtime.RuntimeName{Local: "child"},
		ParentContent: parentContent,
		ParentType:    parent,
	})
	expectXSDCode(t, err, xsderrors.CodeValidationElement)
	if got.Element != runtime.NoElement || got.Type != rt.anyType || !got.Skip || !got.Recover {
		t.Fatalf("ChildStart() = %+v, want recoverable skipped anyType child", got)
	}
}

func TestChildStartSchemaLocationStrictWildcardIsUnsupported(t *testing.T) {
	t.Parallel()

	parent := runtime.ComplexRef(1)
	model := runtime.ContentModelID(4)
	wildcard := runtime.WildcardID(5)
	compiled := runtime.CompiledModel{
		Kind: runtime.CompiledModelDFA,
		Rows: []runtime.CompiledModelRow{
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.WildcardParticle(wildcard, runtime.Occurrence{Min: 1, Max: 1}),
					To:       1,
				}},
			},
			{Accept: true},
		},
	}
	rt := contentRuntimeStub{
		anyType:       runtime.ComplexRef(100),
		childContent:  map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true}},
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
		models:        map[runtime.ContentModelID]runtime.CompiledModel{model: compiled},
		wildcards:     map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}},
	}
	parentContent := runtime.ContentFrameForType(rt, parent).ContentState()
	got, err := ChildStart(rt, ChildInput{
		HasSchemaLocation: func(ns string) bool { return ns == "urn:t" },
		Context:           StartContext{Path: "/root", Line: 2, Column: 3},
		Name:              runtime.RuntimeName{NS: "urn:t", Local: "child"},
		ParentContent:     parentContent,
		ParentType:        parent,
	})
	expectXSDCode(t, err, xsderrors.CodeUnsupportedSchemaHint)
	if got.Recover || got.Skip {
		t.Fatalf("ChildStart() = %+v, want non-recoverable unsupported schemaLocation", got)
	}
}

func TestChildStartStrictWildcardAllowsUndeclaredXSIType(t *testing.T) {
	t.Parallel()

	parent := runtime.ComplexRef(1)
	model := runtime.ContentModelID(4)
	wildcard := runtime.WildcardID(5)
	compiled := runtime.CompiledModel{
		Kind: runtime.CompiledModelDFA,
		Rows: []runtime.CompiledModelRow{
			{
				Edges: []runtime.CompiledModelEdge{{
					Particle: runtime.WildcardParticle(wildcard, runtime.Occurrence{Min: 1, Max: 1}),
					To:       1,
				}},
			},
			{Accept: true},
		},
	}
	rt := contentRuntimeStub{
		anyType:       runtime.ComplexRef(100),
		childContent:  map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true}},
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
		models:        map[runtime.ContentModelID]runtime.CompiledModel{model: compiled},
		wildcards:     map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}},
	}
	parentContent := runtime.ContentFrameForType(rt, parent).ContentState()
	got, err := ChildStart(rt, ChildInput{
		HasSchemaLocation: func(ns string) bool { return ns == "urn:t" },
		Context:           StartContext{Path: "/root", Line: 2, Column: 3},
		Name:              runtime.RuntimeName{NS: "urn:t", Local: "child"},
		ParentContent:     parentContent,
		ParentType:        parent,
		HasXSIType:        true,
	})
	if err != nil {
		t.Fatalf("ChildStart() error = %v", err)
	}
	if got.Element != runtime.NoElement || got.Type != rt.anyType || got.Skip {
		t.Fatalf("ChildStart() = %+v, want undeclared anyType child", got)
	}
}

func TestChildStartInvalidContentStateIsInternalInvariant(t *testing.T) {
	t.Parallel()

	parent := runtime.ComplexRef(1)
	rt := contentRuntimeStub{
		anyType:       runtime.ComplexRef(100),
		childContent:  map[runtime.TypeID]runtime.ChildContentInfo{parent: {Complex: true}},
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: runtime.ContentModelID(4)},
	}
	parentContent := runtime.ContentFrameForType(rt, parent).ContentState()
	_, err := ChildStart(rt, ChildInput{
		Context:       StartContext{Path: "/root", Line: 2, Column: 3},
		Name:          runtime.RuntimeName{Local: "child"},
		ParentContent: parentContent,
		ParentType:    parent,
	})
	expectXSDCode(t, err, xsderrors.CodeInternalInvariant)
}

func TestContentCompleteValidatesEndState(t *testing.T) {
	t.Parallel()

	model := runtime.ContentModelID(4)
	compiled := runtime.CompiledModel{
		Kind:  runtime.CompiledModelDFA,
		Start: 7,
		Rows:  make([]runtime.CompiledModelRow, 8),
	}
	parent := runtime.ComplexRef(1)
	rt := contentRuntimeStub{
		contentModels: map[runtime.TypeID]runtime.ContentModelID{parent: model},
		models:        map[runtime.ContentModelID]runtime.CompiledModel{model: compiled},
	}
	content := runtime.ContentFrameForType(rt, parent).ContentState()
	err := ContentComplete(rt, CompleteInput{
		Context: StartContext{Path: "/root", Line: 2, Column: 3},
		Type:    parent,
		Content: content,
	})
	expectXSDCode(t, err, xsderrors.CodeValidationContent)
}

func TestContentCompleteSkipsNilledContentModel(t *testing.T) {
	t.Parallel()

	called := false
	rt := contentRuntimeStub{
		compiledContentModel: func(runtime.ContentModelID) (runtime.CompiledModel, bool) {
			called = true
			return runtime.CompiledModel{}, false
		},
	}
	err := ContentComplete(rt, CompleteInput{
		Type:   runtime.ComplexRef(1),
		Nilled: true,
	})
	if err != nil {
		t.Fatalf("ContentComplete() error = %v", err)
	}
	if called {
		t.Fatal("ContentComplete() inspected content model for nilled input")
	}
}

func TestContentCompleteIgnoresNonComplexOrAbsentModel(t *testing.T) {
	t.Parallel()

	called := false
	rt := contentRuntimeStub{
		compiledContentModel: func(runtime.ContentModelID) (runtime.CompiledModel, bool) {
			called = true
			return runtime.CompiledModel{}, true
		},
	}
	for _, in := range []CompleteInput{
		{Type: runtime.SimpleRef(1)},
		{Type: runtime.ComplexRef(1)},
	} {
		if err := ContentComplete(rt, in); err != nil {
			t.Fatalf("ContentComplete() error = %v", err)
		}
	}
	if called {
		t.Fatalf("ContentComplete() called runtime for non-content input")
	}
}
