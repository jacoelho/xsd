package schemair

import (
	"cmp"
	"fmt"
	"slices"

	ast "github.com/jacoelho/xsd/internal/schemaast"
)

type docResolver struct {
	docResolverDocs
	docResolverGlobals
	docResolverDecls
	docResolverIDs
	docResolverState
	docResolverIdentity
}

type docResolverDocs struct {
	docs []ast.SchemaDocument
	out  Schema

	contexts map[ast.NamespaceContextID]ast.NamespaceContext
}

type docResolverGlobals struct {
	builtins map[Name]TypeRef
	globals  globalIndex
}

type globalIndex struct {
	typeNames       map[Name]struct{}
	groups          map[Name]*ast.GroupDecl
	attributeGroups map[Name]*ast.AttributeGroupDecl
	notations       map[Name]struct{}
	elementDecls    map[Name]*ast.ElementDecl
	attributeDecls  map[Name]*ast.AttributeDecl
}

type docResolverDecls struct {
	simpleDecls    map[simpleDeclID]*ast.SimpleTypeDecl
	complexDecls   map[complexDeclID]*ast.ComplexTypeDecl
	elementDecls   map[elementDeclID]*ast.ElementDecl
	attributeDecls map[attributeDeclID]*ast.AttributeDecl

	simpleDeclHandles    map[*ast.SimpleTypeDecl]simpleDeclID
	complexDeclHandles   map[*ast.ComplexTypeDecl]complexDeclID
	elementDeclHandles   map[*ast.ElementDecl]elementDeclID
	attributeDeclHandles map[*ast.AttributeDecl]attributeDeclID
}

type docResolverIDs struct {
	ids idPlan
}

type idPlan struct {
	simpleTypes      map[simpleDeclID]TypeID
	simpleByID       map[TypeID]simpleDeclID
	complexTypes     map[complexDeclID]TypeID
	complexByID      map[TypeID]complexDeclID
	elements         map[elementDeclID]ElementID
	elementByID      map[ElementID]elementDeclID
	attributes       map[attributeDeclID]AttributeID
	attributeByID    map[AttributeID]attributeDeclID
	globalTypes      map[Name]TypeID
	globalElements   map[Name]ElementID
	globalAttributes map[Name]AttributeID
}

func newIDPlan() idPlan {
	return idPlan{
		simpleTypes:      make(map[simpleDeclID]TypeID),
		simpleByID:       make(map[TypeID]simpleDeclID),
		complexTypes:     make(map[complexDeclID]TypeID),
		complexByID:      make(map[TypeID]complexDeclID),
		elements:         make(map[elementDeclID]ElementID),
		elementByID:      make(map[ElementID]elementDeclID),
		attributes:       make(map[attributeDeclID]AttributeID),
		attributeByID:    make(map[AttributeID]attributeDeclID),
		globalTypes:      make(map[Name]TypeID),
		globalElements:   make(map[Name]ElementID),
		globalAttributes: make(map[Name]AttributeID),
	}
}

type idPlanBuilder struct {
	resolver *docResolver
	globals  globalIndex
	plan     idPlan

	assignedSimpleTrees   map[simpleDeclID]bool
	assigningSimpleTrees  map[simpleDeclID]bool
	assignedComplexTrees  map[complexDeclID]bool
	assigningComplexTrees map[complexDeclID]bool

	nextType TypeID
	nextElem ElementID
	nextAttr AttributeID
}

type docResolverState struct {
	emittedTypes   map[TypeID]bool
	emittingTypes  map[TypeID]bool
	emittedComplex map[TypeID]bool
	emittedElems   map[ElementID]bool
	emittedAttrs   map[AttributeID]bool
}

type docResolverIdentity struct {
	identityNames map[Name]IdentityID
}

type simpleDeclID uint32
type complexDeclID uint32
type elementDeclID uint32
type attributeDeclID uint32

func (r *docResolver) simpleDeclHandle(decl *ast.SimpleTypeDecl) simpleDeclID {
	if decl == nil {
		return 0
	}
	if id, ok := r.simpleDeclHandles[decl]; ok {
		return id
	}
	id := simpleDeclID(len(r.simpleDecls) + 1)
	r.simpleDecls[id] = decl
	r.simpleDeclHandles[decl] = id
	return id
}

func (r *docResolver) complexDeclHandle(decl *ast.ComplexTypeDecl) complexDeclID {
	if decl == nil {
		return 0
	}
	if id, ok := r.complexDeclHandles[decl]; ok {
		return id
	}
	id := complexDeclID(len(r.complexDecls) + 1)
	r.complexDecls[id] = decl
	r.complexDeclHandles[decl] = id
	return id
}

func (r *docResolver) elementDeclHandle(decl *ast.ElementDecl) elementDeclID {
	if decl == nil {
		return 0
	}
	if id, ok := r.elementDeclHandles[decl]; ok {
		return id
	}
	id := elementDeclID(len(r.elementDecls) + 1)
	r.elementDecls[id] = decl
	r.elementDeclHandles[decl] = id
	return id
}

func (r *docResolver) attributeDeclHandle(decl *ast.AttributeDecl) attributeDeclID {
	if decl == nil {
		return 0
	}
	if id, ok := r.attributeDeclHandles[decl]; ok {
		return id
	}
	id := attributeDeclID(len(r.attributeDecls) + 1)
	r.attributeDecls[id] = decl
	r.attributeDeclHandles[decl] = id
	return id
}

func resolveDocuments(docs *ast.DocumentSet) (*Schema, error) {
	if docs == nil {
		return nil, fmt.Errorf("schema ir: document set is nil")
	}
	if len(docs.Documents) == 0 {
		return nil, fmt.Errorf("schema ir: document set is empty")
	}
	normalizedDocs, contexts := normalizeDocumentContextIDs(docs.Documents)
	r := newDocResolver(normalizedDocs, contexts)
	if err := r.resolve(); err != nil {
		return nil, err
	}
	return &r.out, nil
}

func newDocResolver(docs []ast.SchemaDocument, contexts map[ast.NamespaceContextID]ast.NamespaceContext) *docResolver {
	return &docResolver{
		docResolverDocs: docResolverDocs{
			docs:     docs,
			contexts: contexts,
		},
		docResolverGlobals: docResolverGlobals{
			builtins: make(map[Name]TypeRef),
		},
		docResolverDecls: docResolverDecls{
			simpleDecls:          make(map[simpleDeclID]*ast.SimpleTypeDecl),
			complexDecls:         make(map[complexDeclID]*ast.ComplexTypeDecl),
			elementDecls:         make(map[elementDeclID]*ast.ElementDecl),
			attributeDecls:       make(map[attributeDeclID]*ast.AttributeDecl),
			simpleDeclHandles:    make(map[*ast.SimpleTypeDecl]simpleDeclID),
			complexDeclHandles:   make(map[*ast.ComplexTypeDecl]complexDeclID),
			elementDeclHandles:   make(map[*ast.ElementDecl]elementDeclID),
			attributeDeclHandles: make(map[*ast.AttributeDecl]attributeDeclID),
		},
		docResolverState: docResolverState{
			emittedTypes:   make(map[TypeID]bool),
			emittingTypes:  make(map[TypeID]bool),
			emittedComplex: make(map[TypeID]bool),
			emittedElems:   make(map[ElementID]bool),
			emittedAttrs:   make(map[AttributeID]bool),
		},
		docResolverIdentity: docResolverIdentity{
			identityNames: make(map[Name]IdentityID),
		},
	}
}

func (r *docResolver) resolve() error {
	r.emitBuiltins()
	if err := r.validateImportVisibility(); err != nil {
		return err
	}
	globals, err := buildGlobalIndex(r.docs)
	if err != nil {
		return err
	}
	r.globals = globals
	r.ids = r.buildIDPlan()
	if err := r.validateTypeDerivationCycles(); err != nil {
		return err
	}
	if err := r.emitGlobalDeclarations(); err != nil {
		return err
	}
	restrictions := buildParticleRestrictionPhase(particleRestrictionPhaseInput{
		Types: r.out.Types,
		Plans: r.out.ComplexTypes,
	})
	if err := restrictions.validate(r); err != nil {
		return err
	}
	r.sortComponents()
	if err := validateSubstitutionCycles(substitutionCyclePhaseInput{
		Elements: r.out.Elements,
		Globals:  r.globals,
	}); err != nil {
		return err
	}
	identityRefs, err := resolveIdentityReferences(identityReferencePhaseInput{
		Constraints: r.out.IdentityConstraints,
		Elements:    r.out.Elements,
	}, r.identityFieldTypesCompatible)
	if err != nil {
		return err
	}
	r.out.IdentityConstraints = identityRefs.Constraints
	metadata := buildRuntimeEmission(runtimeEmissionPhaseInput{
		BuiltinTypes:        r.out.BuiltinTypes,
		Types:               r.out.Types,
		Elements:            r.out.Elements,
		Attributes:          r.out.Attributes,
		AttributeUses:       r.out.AttributeUses,
		ComplexTypes:        r.out.ComplexTypes,
		Particles:           r.out.Particles,
		Wildcards:           r.out.Wildcards,
		IdentityConstraints: r.out.IdentityConstraints,
		Docs:                r.docs,
		Globals:             r.globals,
		IDs:                 r.ids,
	})
	r.out.ElementRefs = metadata.ElementRefs
	r.out.AttributeRefs = metadata.AttributeRefs
	r.out.GroupRefs = metadata.GroupRefs
	r.out.GlobalIndexes = metadata.GlobalIndexes
	r.out.RuntimeNames = metadata.RuntimeNames
	r.out.Names = metadata.Names
	return nil
}

func (r *docResolver) sortComponents() {
	slices.SortFunc(r.out.Types, func(a, b TypeDecl) int {
		return cmp.Compare(a.ID, b.ID)
	})
	slices.SortFunc(r.out.SimpleTypes, func(a, b SimpleTypeSpec) int {
		return cmp.Compare(a.TypeDecl, b.TypeDecl)
	})
	slices.SortFunc(r.out.Elements, func(a, b Element) int {
		return cmp.Compare(a.ID, b.ID)
	})
	slices.SortFunc(r.out.Attributes, func(a, b Attribute) int {
		return cmp.Compare(a.ID, b.ID)
	})
	slices.SortFunc(r.out.AttributeUses, func(a, b AttributeUse) int {
		return cmp.Compare(a.ID, b.ID)
	})
	slices.SortFunc(r.out.ComplexTypes, func(a, b ComplexTypePlan) int {
		return cmp.Compare(a.TypeDecl, b.TypeDecl)
	})
}

type substitutionCyclePhaseInput struct {
	Elements []Element
	Globals  globalIndex
}

func validateSubstitutionCycles(input substitutionCyclePhaseInput) error {
	elements := make(map[ElementID]Element, len(input.Elements))
	starts := make([]ElementID, 0, len(input.Elements))
	for _, elem := range input.Elements {
		elements[elem.ID] = elem
		if elem.SubstitutionHead != 0 {
			starts = append(starts, elem.ID)
		}
	}
	slices.Sort(starts)

	state := make(map[ElementID]uint8, len(starts))
	var visit func(ElementID) (ElementID, bool)
	visit = func(id ElementID) (ElementID, bool) {
		switch state[id] {
		case 1:
			return id, true
		case 2:
			return 0, false
		}
		state[id] = 1
		elem, ok := elements[id]
		if ok && elem.SubstitutionHead != 0 {
			if cycle, found := visit(elem.SubstitutionHead); found {
				return cycle, true
			}
		}
		state[id] = 2
		return 0, false
	}

	for _, start := range starts {
		if cycle, found := visit(start); found {
			elem := elements[cycle]
			return fmt.Errorf("schema ir: cyclic substitution group detected: element %s is part of a cycle", formatName(elem.Name))
		}
	}
	return nil
}

func (r *docResolver) emitBuiltins() {
	for i, typ := range builtinTypes() {
		name := nameFromQName(ast.QName{Namespace: ast.XSDNamespace, Local: typ.Name})
		ref := BuiltinTypeRef(TypeID(i+1), name)
		r.builtins[name] = ref

		spec := SimpleTypeSpec{
			Name:            name,
			Builtin:         true,
			Variety:         typ.Variety,
			Primitive:       typ.Primitive,
			BuiltinBase:     typ.Name,
			Whitespace:      typ.Whitespace,
			QNameOrNotation: typ.Name == "QName" || typ.Name == "NOTATION",
			IntegerDerived:  typ.Integer,
		}
		if typ.Base != "" {
			spec.Base = r.builtinRef(typ.Base)
		}
		if typ.Item != "" {
			spec.Item = r.builtinRef(typ.Item)
		}
		r.out.BuiltinTypes = append(r.out.BuiltinTypes, BuiltinType{
			Name:          name,
			Base:          spec.Base,
			AnyType:       typ.Name == "anyType",
			AnySimpleType: typ.Name == "anySimpleType",
			Value:         spec,
		})
		_ = i
	}
}

func buildGlobalIndex(docs []ast.SchemaDocument) (globalIndex, error) {
	index := newGlobalIndex()
	for di := range docs {
		doc := &docs[di]
		for i := range doc.Decls {
			decl := &doc.Decls[i]
			if err := index.addTopLevelDecl(decl); err != nil {
				return globalIndex{}, err
			}
		}
	}
	return index, nil
}

func newGlobalIndex() globalIndex {
	return globalIndex{
		typeNames:       make(map[Name]struct{}),
		groups:          make(map[Name]*ast.GroupDecl),
		attributeGroups: make(map[Name]*ast.AttributeGroupDecl),
		notations:       make(map[Name]struct{}),
		elementDecls:    make(map[Name]*ast.ElementDecl),
		attributeDecls:  make(map[Name]*ast.AttributeDecl),
	}
}

func (g *globalIndex) addTopLevelDecl(decl *ast.TopLevelDecl) error {
	switch decl.Kind {
	case ast.DeclSimpleType:
		if decl.SimpleType == nil {
			return fmt.Errorf("schema ir: simple type declaration %s is nil", formatName(nameFromQName(decl.Name)))
		}
		return g.addSimpleType(decl.SimpleType)
	case ast.DeclComplexType:
		if decl.ComplexType == nil {
			return fmt.Errorf("schema ir: complex type declaration %s is nil", formatName(nameFromQName(decl.Name)))
		}
		return g.addComplexType(decl.ComplexType)
	case ast.DeclElement:
		if decl.Element == nil {
			return fmt.Errorf("schema ir: element declaration %s is nil", formatName(nameFromQName(decl.Name)))
		}
		return g.addElement(decl.Element)
	case ast.DeclAttribute:
		if decl.Attribute == nil {
			return fmt.Errorf("schema ir: attribute declaration %s is nil", formatName(nameFromQName(decl.Name)))
		}
		return g.addAttribute(decl.Attribute)
	case ast.DeclGroup:
		if decl.Group == nil {
			return fmt.Errorf("schema ir: group declaration %s is nil", formatName(nameFromQName(decl.Name)))
		}
		return g.addGroup(decl.Group)
	case ast.DeclAttributeGroup:
		if decl.AttributeGroup == nil {
			return fmt.Errorf("schema ir: attributeGroup declaration %s is nil", formatName(nameFromQName(decl.Name)))
		}
		return g.addAttributeGroup(decl.AttributeGroup)
	case ast.DeclNotation:
		if decl.Notation == nil {
			return fmt.Errorf("schema ir: notation declaration %s is nil", formatName(nameFromQName(decl.Name)))
		}
		return g.addNotation(decl.Notation)
	default:
		return nil
	}
}

func (g *globalIndex) addSimpleType(decl *ast.SimpleTypeDecl) error {
	name := nameFromQName(decl.Name)
	if docIsZeroName(name) {
		return fmt.Errorf("schema ir: global simple type missing name")
	}
	if _, exists := g.typeNames[name]; exists {
		return fmt.Errorf("schema ir: duplicate type %s", formatName(name))
	}
	g.typeNames[name] = struct{}{}
	return nil
}

func (g *globalIndex) addComplexType(decl *ast.ComplexTypeDecl) error {
	name := nameFromQName(decl.Name)
	if docIsZeroName(name) {
		return fmt.Errorf("schema ir: global complex type missing name")
	}
	if _, exists := g.typeNames[name]; exists {
		return fmt.Errorf("schema ir: duplicate type %s", formatName(name))
	}
	g.typeNames[name] = struct{}{}
	return nil
}

func (g *globalIndex) addElement(decl *ast.ElementDecl) error {
	name := nameFromQName(decl.Name)
	if docIsZeroName(name) {
		return fmt.Errorf("schema ir: global element missing name")
	}
	if _, exists := g.elementDecls[name]; exists {
		return fmt.Errorf("schema ir: duplicate element %s", formatName(name))
	}
	g.elementDecls[name] = decl
	return nil
}

func (g *globalIndex) addAttribute(decl *ast.AttributeDecl) error {
	name := nameFromQName(decl.Name)
	if docIsZeroName(name) {
		return fmt.Errorf("schema ir: global attribute missing name")
	}
	if _, exists := g.attributeDecls[name]; exists {
		return fmt.Errorf("schema ir: duplicate attribute %s", formatName(name))
	}
	g.attributeDecls[name] = decl
	return nil
}

func (g *globalIndex) addGroup(decl *ast.GroupDecl) error {
	name := nameFromQName(decl.Name)
	if _, exists := g.groups[name]; exists {
		return fmt.Errorf("schema ir: duplicate group %s", formatName(name))
	}
	g.groups[name] = decl
	return nil
}

func (g *globalIndex) addAttributeGroup(decl *ast.AttributeGroupDecl) error {
	name := nameFromQName(decl.Name)
	if _, exists := g.attributeGroups[name]; exists {
		return fmt.Errorf("schema ir: duplicate attributeGroup %s", formatName(name))
	}
	g.attributeGroups[name] = decl
	return nil
}

func (g *globalIndex) addNotation(decl *ast.NotationDecl) error {
	name := nameFromQName(decl.Name)
	if docIsZeroName(name) {
		return fmt.Errorf("schema ir: notation missing name")
	}
	if _, exists := g.notations[name]; exists {
		return fmt.Errorf("schema ir: duplicate notation %s", formatName(name))
	}
	g.notations[name] = struct{}{}
	return nil
}

func (r *docResolver) buildIDPlan() idPlan {
	builder := idPlanBuilder{
		resolver:              r,
		globals:               r.globals,
		plan:                  newIDPlan(),
		assignedSimpleTrees:   make(map[simpleDeclID]bool),
		assigningSimpleTrees:  make(map[simpleDeclID]bool),
		assignedComplexTrees:  make(map[complexDeclID]bool),
		assigningComplexTrees: make(map[complexDeclID]bool),
		nextType:              1,
		nextElem:              1,
		nextAttr:              1,
	}
	builder.assignDeclarationIDs(r.docs)
	return builder.plan
}

func (b *idPlanBuilder) assignDeclarationIDs(docs []ast.SchemaDocument) {
	for di := range docs {
		doc := &docs[di]
		for i := range doc.Decls {
			decl := &doc.Decls[i]
			switch decl.Kind {
			case ast.DeclSimpleType:
				b.assignSimpleTypeTree(decl.SimpleType, true)
			case ast.DeclComplexType:
				b.assignComplexTypeTree(decl.ComplexType, true)
			case ast.DeclElement:
				b.assignElementID(decl.Element, true)
				if decl.Element != nil {
					b.assignTypeUseInlineIDs(decl.Element.Type)
				}
			case ast.DeclAttribute:
				b.assignAttributeID(decl.Attribute, true)
				if decl.Attribute != nil {
					b.assignTypeUseInlineIDs(decl.Attribute.Type)
				}
			case ast.DeclGroup:
				if decl.Group != nil {
					b.assignParticleInlineIDs(decl.Group.Particle)
				}
			case ast.DeclAttributeGroup:
				if decl.AttributeGroup != nil {
					for i := range decl.AttributeGroup.Attributes {
						b.assignAttributeUseInlineIDs(&decl.AttributeGroup.Attributes[i], true)
					}
				}
			}
		}
	}
}

func (b *idPlanBuilder) assignSimpleTypeTree(decl *ast.SimpleTypeDecl, global bool) {
	if decl == nil {
		return
	}
	handle := b.resolver.simpleDeclHandle(decl)
	if b.assignedSimpleTrees[handle] || b.assigningSimpleTrees[handle] {
		return
	}
	b.assigningSimpleTrees[handle] = true
	defer delete(b.assigningSimpleTrees, handle)
	b.assignSimpleTypeID(decl, global)
	b.assignSimpleTypeTree(decl.InlineBase, false)
	b.assignSimpleTypeTree(decl.InlineItem, false)
	for i := range decl.InlineMembers {
		b.assignSimpleTypeTree(&decl.InlineMembers[i], false)
	}
	b.assignedSimpleTrees[handle] = true
}

func (b *idPlanBuilder) assignSimpleTypeID(decl *ast.SimpleTypeDecl, global bool) {
	if decl == nil {
		return
	}
	handle := b.resolver.simpleDeclHandle(decl)
	if _, exists := b.plan.simpleTypes[handle]; exists {
		return
	}
	id := b.nextType
	b.nextType++
	b.plan.simpleTypes[handle] = id
	b.plan.simpleByID[id] = handle
	if global {
		b.plan.globalTypes[nameFromQName(decl.Name)] = id
	}
}

func (b *idPlanBuilder) assignComplexTypeTree(decl *ast.ComplexTypeDecl, global bool) {
	if decl == nil {
		return
	}
	handle := b.resolver.complexDeclHandle(decl)
	if b.assignedComplexTrees[handle] || b.assigningComplexTrees[handle] {
		return
	}
	b.assigningComplexTrees[handle] = true
	defer delete(b.assigningComplexTrees, handle)
	b.assignComplexTypeID(decl, global)
	b.assignSimpleTypeTree(decl.SimpleType, false)
	b.assignParticleInlineIDs(decl.Particle)
	for i := range decl.Attributes {
		b.assignAttributeUseInlineIDs(&decl.Attributes[i], false)
	}
	b.assignedComplexTrees[handle] = true
}

func (r *docResolver) validateTypeDerivationCycles() error {
	state := make(map[TypeID]uint8, len(r.ids.simpleByID)+len(r.ids.complexByID))
	ids := make([]TypeID, 0, len(r.ids.simpleByID)+len(r.ids.complexByID))
	for id := range r.ids.simpleByID {
		ids = append(ids, id)
	}
	for id := range r.ids.complexByID {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	var visit func(TypeID) error
	visit = func(id TypeID) error {
		switch state[id] {
		case 1:
			return fmt.Errorf("schema ir: type derivation cycle at %s", formatName(r.typeNameByID(id)))
		case 2:
			return nil
		}
		state[id] = 1
		for _, ref := range r.typeDerivationRefs(id) {
			refID := ref.TypeID()
			if ref.IsBuiltin() || refID == 0 {
				continue
			}
			if err := visit(refID); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}

	for _, id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func (r *docResolver) typeDerivationRefs(id TypeID) []TypeRef {
	if decl := r.simpleDecls[r.ids.simpleByID[id]]; decl != nil {
		return r.simpleDerivationRefs(decl)
	}
	if decl := r.complexDecls[r.ids.complexByID[id]]; decl != nil {
		if decl.Base.IsZero() {
			return nil
		}
		refs := make([]TypeRef, 0, 1)
		return append(refs, r.typeRefZero(decl.Base))
	}
	return nil
}

func (r *docResolver) simpleDerivationRefs(decl *ast.SimpleTypeDecl) []TypeRef {
	if decl == nil {
		return nil
	}
	var refs []TypeRef
	switch decl.Kind {
	case ast.SimpleDerivationRestriction:
		if !decl.Base.IsZero() {
			refs = append(refs, r.typeRefZero(decl.Base))
		}
		if decl.InlineBase != nil {
			refs = append(refs, UserTypeRef(r.ids.simpleTypes[r.simpleDeclHandle(decl.InlineBase)], nameFromQName(decl.InlineBase.Name)))
		}
	case ast.SimpleDerivationList:
		if !decl.ItemType.IsZero() {
			refs = append(refs, r.typeRefZero(decl.ItemType))
		}
		if decl.InlineItem != nil {
			refs = append(refs, UserTypeRef(r.ids.simpleTypes[r.simpleDeclHandle(decl.InlineItem)], nameFromQName(decl.InlineItem.Name)))
		}
	case ast.SimpleDerivationUnion:
		for _, member := range decl.MemberTypes {
			refs = append(refs, r.typeRefZero(member))
		}
		for i := range decl.InlineMembers {
			member := &decl.InlineMembers[i]
			refs = append(refs, UserTypeRef(r.ids.simpleTypes[r.simpleDeclHandle(member)], nameFromQName(member.Name)))
		}
	}
	return refs
}

func (r *docResolver) typeNameByID(id TypeID) Name {
	if decl := r.simpleDecls[r.ids.simpleByID[id]]; decl != nil {
		return nameFromQName(decl.Name)
	}
	if decl := r.complexDecls[r.ids.complexByID[id]]; decl != nil {
		return nameFromQName(decl.Name)
	}
	return Name{}
}

func (b *idPlanBuilder) assignComplexTypeID(decl *ast.ComplexTypeDecl, global bool) {
	if decl == nil {
		return
	}
	handle := b.resolver.complexDeclHandle(decl)
	if _, exists := b.plan.complexTypes[handle]; exists {
		return
	}
	id := b.nextType
	b.nextType++
	b.plan.complexTypes[handle] = id
	b.plan.complexByID[id] = handle
	if global {
		b.plan.globalTypes[nameFromQName(decl.Name)] = id
	}
}

func (b *idPlanBuilder) assignTypeUseInlineIDs(use ast.TypeUse) {
	b.assignSimpleTypeTree(use.Simple, false)
	b.assignComplexTypeTree(use.Complex, false)
}

func (b *idPlanBuilder) assignParticleInlineIDs(decl *ast.ParticleDecl) {
	b.assignParticleInlineIDsWithStack(decl, nil)
}

func (b *idPlanBuilder) assignParticleInlineIDsWithStack(decl *ast.ParticleDecl, stack []Name) {
	if decl == nil {
		return
	}
	if decl.Kind == ast.ParticleGroup {
		groupName := nameFromQName(decl.GroupRef)
		if slices.Contains(stack, groupName) {
			return
		}
		group := b.globals.groups[groupName]
		if group != nil {
			b.assignParticleInlineIDsWithStack(group.Particle, append(stack, groupName))
		}
		return
	}
	if decl.Element != nil {
		b.assignElementID(decl.Element, false)
		b.assignTypeUseInlineIDs(decl.Element.Type)
	}
	for i := range decl.Children {
		b.assignParticleInlineIDsWithStack(&decl.Children[i], stack)
	}
}

func (b *idPlanBuilder) assignAttributeUseInlineIDs(use *ast.AttributeUseDecl, emitLocalDecl bool) {
	if use == nil || use.Attribute == nil {
		return
	}
	if emitLocalDecl {
		b.assignAttributeID(use.Attribute, false)
	}
	b.assignTypeUseInlineIDs(use.Attribute.Type)
}

func (b *idPlanBuilder) assignElementID(decl *ast.ElementDecl, global bool) {
	if decl == nil || !decl.Ref.IsZero() {
		return
	}
	handle := b.resolver.elementDeclHandle(decl)
	if _, exists := b.plan.elements[handle]; exists {
		return
	}
	id := b.nextElem
	b.nextElem++
	b.plan.elements[handle] = id
	b.plan.elementByID[id] = handle
	if global {
		b.plan.globalElements[nameFromQName(decl.Name)] = id
	}
}

func (b *idPlanBuilder) assignAttributeID(decl *ast.AttributeDecl, global bool) {
	if decl == nil || !decl.Ref.IsZero() {
		return
	}
	handle := b.resolver.attributeDeclHandle(decl)
	if _, exists := b.plan.attributes[handle]; exists {
		return
	}
	id := b.nextAttr
	b.nextAttr++
	b.plan.attributes[handle] = id
	b.plan.attributeByID[id] = handle
	if global {
		b.plan.globalAttributes[nameFromQName(decl.Name)] = id
	}
}

func (r *docResolver) emitGlobalDeclarations() error {
	for di := range r.docs {
		for i := range r.docs[di].Decls {
			decl := &r.docs[di].Decls[i]
			switch decl.Kind {
			case ast.DeclSimpleType:
				if _, err := r.ensureSimpleType(decl.SimpleType, true); err != nil {
					return err
				}
			case ast.DeclComplexType:
				if _, err := r.ensureComplexType(decl.ComplexType, true); err != nil {
					return err
				}
			case ast.DeclElement:
				if _, err := r.ensureElement(decl.Element, true); err != nil {
					return err
				}
			case ast.DeclAttribute:
				if _, err := r.ensureAttribute(decl.Attribute, true); err != nil {
					return err
				}
			case ast.DeclGroup:
				if err := r.emitGroupDeclarations(decl.Group, nil); err != nil {
					return err
				}
			case ast.DeclAttributeGroup:
				if err := r.emitAttributeGroupDeclarations(decl.AttributeGroup, nil); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *docResolver) emitGroupDeclarations(group *ast.GroupDecl, stack []Name) error {
	if group == nil {
		return nil
	}
	name := nameFromQName(group.Name)
	if slices.Contains(stack, name) {
		return fmt.Errorf("schema ir: group ref cycle detected: %s", formatName(name))
	}
	return r.emitParticleDeclarations(group.Particle, append(stack, name))
}

func (r *docResolver) emitParticleDeclarations(decl *ast.ParticleDecl, stack []Name) error {
	if decl == nil {
		return nil
	}
	if decl.Kind == ast.ParticleGroup {
		groupName := nameFromQName(decl.GroupRef)
		group, ok := r.globals.groups[groupName]
		if !ok {
			return fmt.Errorf("schema ir: group ref %s not found", formatName(groupName))
		}
		return r.emitGroupDeclarations(group, stack)
	}
	if decl.Element != nil && decl.Element.Ref.IsZero() {
		if _, err := r.ensureElement(decl.Element, false); err != nil {
			return err
		}
	}
	for i := range decl.Children {
		if err := r.emitParticleDeclarations(&decl.Children[i], stack); err != nil {
			return err
		}
	}
	return nil
}

func (r *docResolver) emitAttributeGroupDeclarations(group *ast.AttributeGroupDecl, stack []Name) error {
	if group == nil {
		return nil
	}
	name := nameFromQName(group.Name)
	if slices.Contains(stack, name) {
		return fmt.Errorf("schema ir: attributeGroup ref cycle detected: %s", formatName(name))
	}
	stack = append(stack, name)
	for i := range group.Attributes {
		use := &group.Attributes[i]
		if use.Attribute == nil || !use.Attribute.Ref.IsZero() {
			continue
		}
		if _, err := r.ensureAttribute(use.Attribute, false); err != nil {
			return err
		}
	}
	for _, ref := range group.AttributeGroups {
		nestedName := nameFromQName(ref)
		nested, ok := r.globals.attributeGroups[nestedName]
		if !ok {
			return fmt.Errorf("schema ir: attributeGroup ref %s not found", formatName(nestedName))
		}
		if err := r.emitAttributeGroupDeclarations(nested, stack); err != nil {
			return err
		}
	}
	return nil
}

func (r *docResolver) ensureSimpleType(decl *ast.SimpleTypeDecl, global bool) (TypeID, error) {
	if decl == nil {
		return 0, nil
	}
	handle := r.simpleDeclHandle(decl)
	id, ok := r.ids.simpleTypes[handle]
	return r.ensureType(global, nameFromQName(decl.Name), id, ok, func(id TypeID, global bool) error {
		return r.emitSimpleType(id, decl, global)
	})
}

func (r *docResolver) ensureComplexType(decl *ast.ComplexTypeDecl, global bool) (TypeID, error) {
	if decl == nil {
		return 0, nil
	}
	handle := r.complexDeclHandle(decl)
	id, ok := r.ids.complexTypes[handle]
	return r.ensureType(global, nameFromQName(decl.Name), id, ok, func(id TypeID, global bool) error {
		return r.emitComplexType(id, decl, global)
	})
}

func (r *docResolver) ensureType(
	global bool,
	name Name,
	id TypeID,
	ok bool,
	emit func(TypeID, bool) error,
) (TypeID, error) {
	if !ok {
		return 0, fmt.Errorf("schema ir: type %s missing ID", formatName(name))
	}
	return r.emitPendingType(id, global, name, func(global bool) error {
		return emit(id, global)
	})
}

func (r *docResolver) emitPendingType(id TypeID, global bool, name Name, emit func(bool) error) (TypeID, error) {
	if r.emittedTypes[id] {
		return id, nil
	}
	if r.emittingTypes[id] {
		return 0, fmt.Errorf("schema ir: type derivation cycle at %s", formatName(name))
	}
	if err := emit(global); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *docResolver) emitSimpleType(id TypeID, decl *ast.SimpleTypeDecl, global bool) error {
	r.emittingTypes[id] = true
	defer delete(r.emittingTypes, id)
	name := nameFromQName(decl.Name)
	base, derivation, err := r.simpleBaseAndDerivation(decl)
	if err != nil {
		return err
	}
	if !base.IsBuiltin() && base.TypeID() == id {
		return fmt.Errorf("schema ir: type derivation cycle at %s", formatName(name))
	}
	if err := r.validateSimpleDerivationFinal(decl, base, derivation); err != nil {
		return err
	}
	r.out.Types = append(r.out.Types, TypeDecl{
		ID:         id,
		Name:       name,
		Kind:       TypeSimple,
		Base:       base,
		Derivation: derivation,
		Final:      derivationSet(decl.Final),
		Global:     global,
		Origin:     decl.Origin,
	})
	spec, err := r.simpleSpec(id, name, decl)
	if err != nil {
		return err
	}
	r.out.SimpleTypes = append(r.out.SimpleTypes, spec)
	r.emittedTypes[id] = true
	return nil
}

func (r *docResolver) validateSimpleDerivationFinal(decl *ast.SimpleTypeDecl, base TypeRef, derivation Derivation) error {
	switch decl.Kind {
	case ast.SimpleDerivationRestriction:
		return r.validateSimpleRefFinal(base, derivation, "restrict")
	case ast.SimpleDerivationList:
		item, err := r.simpleItemRef(decl)
		if err != nil {
			return err
		}
		return r.validateSimpleRefFinal(item, DerivationList, "derive list from")
	case ast.SimpleDerivationUnion:
		members, err := r.simpleMemberRefs(decl)
		if err != nil {
			return err
		}
		for _, member := range members {
			if err := r.validateSimpleRefFinal(member, DerivationUnion, "derive union from"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *docResolver) validateSimpleRefFinal(ref TypeRef, derivation Derivation, action string) error {
	if ref.IsZero() || ref.IsBuiltin() || derivation == DerivationNone {
		return nil
	}
	baseDecl, ok, err := r.typeInfoForRef(ref)
	if err != nil || !ok {
		return err
	}
	if baseDecl.Final&derivation == 0 {
		return nil
	}
	return fmt.Errorf("schema ir: cannot %s type %s: base type is final for %s", action, formatName(ref.TypeName()), derivationLabel(derivation))
}

func (r *docResolver) emitComplexType(id TypeID, decl *ast.ComplexTypeDecl, global bool) error {
	r.emittingTypes[id] = true
	defer delete(r.emittingTypes, id)
	name := nameFromQName(decl.Name)
	base := r.builtinRef("anyType")
	if !decl.Base.IsZero() {
		resolved, err := r.typeRef(decl.Base)
		if err != nil {
			return err
		}
		if !resolved.IsBuiltin() && resolved.TypeID() == id {
			return fmt.Errorf("schema ir: type derivation cycle at %s", formatName(nameFromQName(decl.Name)))
		}
		base = resolved
	}
	if err := r.validateComplexContentBaseKind(decl, base); err != nil {
		return err
	}
	if err := r.validateComplexDerivationFinal(decl, base); err != nil {
		return err
	}
	derivation := DerivationRestriction
	switch decl.Derivation {
	case ast.ComplexDerivationExtension:
		derivation = DerivationExtension
	case ast.ComplexDerivationRestriction:
		derivation = DerivationRestriction
	}
	r.out.Types = append(r.out.Types, TypeDecl{
		ID:         id,
		Name:       name,
		Kind:       TypeComplex,
		Base:       base,
		Derivation: derivation,
		Final:      derivationSet(decl.Final),
		Block:      derivationSet(decl.Block),
		Abstract:   decl.Abstract,
		Global:     global,
		Origin:     decl.Origin,
	})
	if err := r.emitComplexPlan(id, decl); err != nil {
		return err
	}
	r.emittedTypes[id] = true
	return nil
}

func (r *docResolver) validateComplexContentBaseKind(decl *ast.ComplexTypeDecl, base TypeRef) error {
	if decl == nil || decl.Content != ast.ComplexContentComplex || decl.Derivation == ast.ComplexDerivationNone || isBuiltinAnyType(base) {
		return nil
	}
	info, ok, err := r.typeInfoForRef(base)
	if err != nil || !ok {
		return err
	}
	if info.Kind != TypeSimple {
		return nil
	}
	switch decl.Derivation {
	case ast.ComplexDerivationExtension:
		return fmt.Errorf("schema ir: complexContent extension cannot derive from simpleType '%s'", base.TypeName().Local)
	case ast.ComplexDerivationRestriction:
		return fmt.Errorf("schema ir: complexContent restriction cannot derive from simpleType '%s'", base.TypeName().Local)
	default:
		return nil
	}
}

func (r *docResolver) validateComplexDerivationFinal(decl *ast.ComplexTypeDecl, base TypeRef) error {
	baseID := base.TypeID()
	if decl == nil || base.IsBuiltin() || baseID == 0 {
		return nil
	}
	baseDecl := r.complexDecls[r.ids.complexByID[baseID]]
	if baseDecl == nil {
		return nil
	}
	switch decl.Derivation {
	case ast.ComplexDerivationExtension:
		if baseDecl.Final.Has(ast.DerivationExtension) {
			return fmt.Errorf("schema ir: cannot extend type %s: base type is final for extension", formatName(base.TypeName()))
		}
	case ast.ComplexDerivationRestriction:
		if baseDecl.Final.Has(ast.DerivationRestriction) {
			return fmt.Errorf("schema ir: cannot restrict type %s: base type is final for restriction", formatName(base.TypeName()))
		}
	}
	return nil
}

func (r *docResolver) emitComplexPlan(id TypeID, decl *ast.ComplexTypeDecl) error {
	if r.emittedComplex[id] {
		return nil
	}
	r.emittedComplex[id] = true
	plan := ComplexTypePlan{
		TypeDecl: id,
		Mixed:    decl.Mixed,
		Content:  ContentEmpty,
	}
	base, err := r.applyComplexPlanBase(&plan, decl)
	if err != nil {
		return err
	}
	restrictionAttrs, localAny, hasLocalAny, err := r.applyComplexPlanAttributes(&plan, decl, base)
	if err != nil {
		return err
	}
	if decl.Derivation == ast.ComplexDerivationRestriction && base.hasPlan {
		if err := r.validateAttributeRestriction(base.plan.Attrs, restrictionAttrs, base.inheritedAny, complexAttributeRestrictionContext(decl)); err != nil {
			return err
		}
	}
	if err := r.applyComplexPlanAnyAttribute(&plan, decl, id, base, localAny, hasLocalAny); err != nil {
		return err
	}
	if err := r.applyComplexPlanContent(&plan, decl, base); err != nil {
		return err
	}
	r.sortAttributeUses(plan.Attrs)
	if err := r.validateComplexPlan(plan); err != nil {
		return err
	}
	r.out.ComplexTypes = append(r.out.ComplexTypes, plan)
	return nil
}

type complexPlanBase struct {
	ref          TypeRef
	plan         ComplexTypePlan
	hasPlan      bool
	inheritedAny WildcardID
}

func (r *docResolver) applyComplexPlanBase(plan *ComplexTypePlan, decl *ast.ComplexTypeDecl) (complexPlanBase, error) {
	base := complexPlanBase{ref: r.typeRefZero(decl.Base)}
	if base.ref.IsZero() {
		return base, nil
	}
	var err error
	base.plan, base.hasPlan, err = r.ensureBaseComplexPlan(base.ref)
	if err != nil {
		return base, err
	}
	if isBuiltinAnyType(base.ref) && decl.Derivation == ast.ComplexDerivationExtension && decl.Content == ast.ComplexContentComplex {
		plan.Particle = r.addAnyTypeContentParticle()
		plan.Content = ContentElement
	}
	if !base.hasPlan {
		return base, nil
	}
	switch decl.Derivation {
	case ast.ComplexDerivationExtension:
		plan.Attrs = r.appendNonProhibitedAttributeUses(plan.Attrs, base.plan.Attrs)
		base.inheritedAny = base.plan.AnyAttr
		if base.plan.Content != ContentSimple || decl.Content == ast.ComplexContentSimple {
			copyComplexPlanContent(plan, base.plan)
		}
		if decl.Particle != nil && base.plan.Content == ContentSimple {
			return base, fmt.Errorf("schema ir: cannot extend simpleContent type %s with particles", formatName(base.ref.TypeName()))
		}
	case ast.ComplexDerivationRestriction:
		plan.Attrs = append(plan.Attrs, base.plan.Attrs...)
		base.inheritedAny = base.plan.AnyAttr
	}
	if decl.Content == ast.ComplexContentSimple && plan.TextType.IsZero() {
		plan.TextType = base.plan.TextType
	}
	if decl.Content == ast.ComplexContentComplex {
		if err := r.validateMixedContentDerivation(decl, base.ref, base.plan); err != nil {
			return base, err
		}
		if decl.Derivation == ast.ComplexDerivationRestriction && base.plan.Content == ContentSimple {
			return base, fmt.Errorf("schema ir: complexContent restriction cannot derive from simpleContent type '%s'", base.ref.TypeName().Local)
		}
	}
	return base, nil
}

func copyComplexPlanContent(dst *ComplexTypePlan, src ComplexTypePlan) {
	dst.Particle = src.Particle
	dst.Content = src.Content
	dst.TextType = src.TextType
	dst.TextSpec = src.TextSpec
}

func (r *docResolver) applyComplexPlanAttributes(
	plan *ComplexTypePlan,
	decl *ast.ComplexTypeDecl,
	base complexPlanBase,
) ([]AttributeUseID, WildcardID, bool, error) {
	var restrictionAttrs []AttributeUseID
	isRestriction := decl.Derivation == ast.ComplexDerivationRestriction
	trackRestriction := isRestriction && base.hasPlan
	for i := range decl.Attributes {
		ids, err := r.attributeUses(&decl.Attributes[i], nil, false)
		if err != nil {
			return nil, 0, false, err
		}
		if trackRestriction {
			restrictionAttrs = append(restrictionAttrs, ids...)
		}
		plan.Attrs = r.appendAttributeUses(plan.Attrs, ids, isRestriction)
	}
	restrictionAttrs, localAny, hasLocalAny, err := r.applyComplexPlanAttributeGroups(plan, decl, trackRestriction, isRestriction, restrictionAttrs)
	if err != nil {
		return nil, 0, false, err
	}
	if decl.AnyAttribute != nil {
		own := r.addWildcard(decl.AnyAttribute)
		localAny, hasLocalAny = r.intersectLocalWildcards(localAny, hasLocalAny, own)
	}
	return restrictionAttrs, localAny, hasLocalAny, nil
}

func (r *docResolver) applyComplexPlanAttributeGroups(
	plan *ComplexTypePlan,
	decl *ast.ComplexTypeDecl,
	trackRestriction bool,
	isRestriction bool,
	restrictionAttrs []AttributeUseID,
) ([]AttributeUseID, WildcardID, bool, error) {
	var localAny WildcardID
	var hasLocalAny bool
	for _, group := range decl.AttributeGroups {
		groupName := nameFromQName(group)
		ids, err := r.attributeGroupUses(groupName, nil)
		if err != nil {
			return nil, 0, false, err
		}
		if trackRestriction {
			restrictionAttrs = append(restrictionAttrs, ids...)
		}
		plan.Attrs = r.appendAttributeUses(plan.Attrs, ids, isRestriction)
		groupWildcard, hasGroupWildcard, err := r.attributeGroupWildcard(groupName, nil)
		if err != nil {
			return nil, 0, false, err
		}
		if hasGroupWildcard {
			localAny, hasLocalAny = r.intersectLocalWildcards(localAny, hasLocalAny, groupWildcard)
		}
	}
	return restrictionAttrs, localAny, hasLocalAny, nil
}

func (r *docResolver) applyComplexPlanAnyAttribute(
	plan *ComplexTypePlan,
	decl *ast.ComplexTypeDecl,
	id TypeID,
	base complexPlanBase,
	localAny WildcardID,
	hasLocalAny bool,
) error {
	switch decl.Derivation {
	case ast.ComplexDerivationExtension:
		if hasLocalAny {
			anyAttr, err := r.unionWildcards(localAny, base.inheritedAny)
			if err != nil {
				return err
			}
			plan.AnyAttr = anyAttr
		} else if base.inheritedAny != 0 {
			plan.AnyAttr = r.cloneWildcard(base.inheritedAny)
		} else {
			plan.AnyAttr = 0
		}
	case ast.ComplexDerivationRestriction:
		if hasLocalAny && localAny != 0 {
			if base.inheritedAny == 0 {
				return fmt.Errorf("schema ir: complex type %d restricts absent anyAttribute", id)
			}
			if err := r.validateAnyAttributeRestriction(base.inheritedAny, localAny); err != nil {
				return err
			}
			plan.AnyAttr = r.intersectWildcards(base.inheritedAny, localAny)
		}
	default:
		if hasLocalAny {
			plan.AnyAttr = localAny
		}
	}
	return nil
}

func (r *docResolver) applyComplexPlanContent(plan *ComplexTypePlan, decl *ast.ComplexTypeDecl, base complexPlanBase) error {
	if decl.Content == ast.ComplexContentSimple {
		plan.Content = ContentSimple
		textType, textSpec, err := r.simpleContentText(decl, base.ref, base.plan, base.hasPlan)
		if err != nil {
			return err
		}
		plan.TextType = textType
		plan.TextSpec = textSpec
		return nil
	}
	if decl.Particle == nil {
		return nil
	}
	return r.applyComplexPlanParticleContent(plan, decl, base)
}

func (r *docResolver) applyComplexPlanParticleContent(plan *ComplexTypePlan, decl *ast.ComplexTypeDecl, base complexPlanBase) error {
	if decl.Derivation == ast.ComplexDerivationExtension && decl.Particle.Kind == ast.ParticleAll && base.hasPlan && base.plan.Content != ContentAll && base.plan.Particle != 0 {
		return fmt.Errorf("schema ir: cannot extend non-empty content model with all content model")
	}
	particle, err := r.addParticle(decl.Particle, nil)
	if err != nil {
		return err
	}
	if err := r.validateComplexPlanAllExtension(base, decl, particle); err != nil {
		return err
	}
	if decl.Derivation == ast.ComplexDerivationExtension && plan.Particle != 0 && particle != 0 {
		plan.Particle = r.addSequenceParticle(plan.Particle, particle)
	} else {
		plan.Particle = particle
	}
	if decl.Particle.Kind == ast.ParticleAll && decl.Derivation != ast.ComplexDerivationExtension {
		plan.Content = ContentAll
	} else if plan.Particle != 0 {
		plan.Content = ContentElement
	}
	return nil
}

func (r *docResolver) validateComplexPlanAllExtension(base complexPlanBase, decl *ast.ComplexTypeDecl, particle ParticleID) error {
	if decl.Derivation != ast.ComplexDerivationExtension || base.plan.Content != ContentAll || particle == 0 {
		return nil
	}
	count, err := r.activeParticleChildCount(base.plan.Particle)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("schema ir: cannot extend all content model with additional particles")
	}
	return nil
}

func (r *docResolver) validateMixedContentDerivation(decl *ast.ComplexTypeDecl, baseRef TypeRef, basePlan ComplexTypePlan) error {
	if decl == nil {
		return nil
	}
	baseMixed := basePlan.Mixed
	derivedMixed := decl.Mixed
	switch decl.Derivation {
	case ast.ComplexDerivationExtension:
		if baseMixed && !derivedMixed && emptyExtensionParticle(decl.Particle) {
			return nil
		}
		if baseMixed && !derivedMixed {
			return fmt.Errorf("schema ir: mixed content derivation: cannot extend mixed content type '%s' to element-only content", baseRef.TypeName().Local)
		}
		if !baseMixed && derivedMixed {
			return fmt.Errorf("schema ir: mixed content derivation: cannot extend element-only content type '%s' to mixed content", baseRef.TypeName().Local)
		}
	case ast.ComplexDerivationRestriction:
		if !baseMixed && derivedMixed {
			return fmt.Errorf("schema ir: mixed content derivation: cannot restrict element-only content type '%s' to mixed content", baseRef.TypeName().Local)
		}
	}
	return nil
}

func emptyExtensionParticle(p *ast.ParticleDecl) bool {
	if p == nil {
		return true
	}
	return p.Kind == ast.ParticleGroup && len(p.Children) == 0
}

func (r *docResolver) simpleContentText(
	decl *ast.ComplexTypeDecl,
	baseRef TypeRef,
	basePlan ComplexTypePlan,
	hasBasePlan bool,
) (TypeRef, SimpleTypeSpec, error) {
	if decl.Derivation == ast.ComplexDerivationExtension && isBuiltinAnyType(baseRef) {
		return NoTypeRef(), SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent extension cannot have base type anyType")
	}
	baseTextRef, baseTextSpec, err := r.simpleContentBaseText(baseRef, basePlan, hasBasePlan)
	if err != nil {
		return NoTypeRef(), SimpleTypeSpec{}, err
	}
	if decl.Derivation != ast.ComplexDerivationRestriction {
		return baseTextRef, baseTextSpec, nil
	}
	if err := r.validateSimpleContentRestrictionBase(baseRef); err != nil {
		return NoTypeRef(), SimpleTypeSpec{}, err
	}

	baseSpec, ok, err := r.simpleContentBaseSpec(baseTextRef, baseTextSpec)
	if err != nil {
		return NoTypeRef(), SimpleTypeSpec{}, err
	}
	if !ok {
		return NoTypeRef(), SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent restriction base %s has no simple value type", formatName(baseRef.TypeName()))
	}
	spec, err := r.simpleContentRestrictionSpec(decl, baseTextRef, baseSpec)
	if err != nil {
		return NoTypeRef(), SimpleTypeSpec{}, err
	}
	spec, err = r.applyRestrictionFacets(spec, decl.SimpleFacets, restrictionFacetOptions{
		context:               "simpleContent restriction",
		rejectAnySimpleFacets: true,
	})
	if err != nil {
		return NoTypeRef(), SimpleTypeSpec{}, err
	}
	return NoTypeRef(), spec, nil
}

func (r *docResolver) validateSimpleContentRestrictionBase(baseRef TypeRef) error {
	if baseRef.IsZero() {
		return nil
	}
	if baseRef.IsBuiltin() {
		return fmt.Errorf("schema ir: simpleContent restriction cannot have simpleType base '%s'", formatName(baseRef.TypeName()))
	}
	if r.simpleDecls[r.ids.simpleByID[baseRef.TypeID()]] != nil {
		return fmt.Errorf("schema ir: simpleContent restriction cannot have simpleType base '%s'", formatName(baseRef.TypeName()))
	}
	return nil
}

func (r *docResolver) simpleContentRestrictionSpec(
	decl *ast.ComplexTypeDecl,
	baseTextRef TypeRef,
	baseSpec SimpleTypeSpec,
) (SimpleTypeSpec, error) {
	specRef := baseTextRef
	source := baseSpec
	if decl.SimpleType != nil {
		inlineID, err := r.ensureSimpleType(decl.SimpleType, false)
		if err != nil {
			return SimpleTypeSpec{}, err
		}
		inlineRef := UserTypeRef(inlineID, nameFromQName(decl.SimpleType.Name))
		if err := r.validateSimpleContentInlineRestriction(baseTextRef, inlineRef); err != nil {
			return SimpleTypeSpec{}, err
		}
		inlineSpec, ok, err := r.simpleContentBaseSpec(inlineRef, SimpleTypeSpec{})
		if err != nil {
			return SimpleTypeSpec{}, err
		}
		if !ok {
			return SimpleTypeSpec{}, fmt.Errorf("schema ir: nested simpleContent simpleType %d has no simple value type", inlineID)
		}
		specRef = inlineRef
		source = inlineSpec
	}
	spec := SimpleTypeSpec{
		Base:            specRef,
		Variety:         source.Variety,
		Item:            source.Item,
		Primitive:       source.Primitive,
		BuiltinBase:     source.BuiltinBase,
		Whitespace:      source.Whitespace,
		QNameOrNotation: source.QNameOrNotation,
		IntegerDerived:  source.IntegerDerived,
	}
	spec.Members = append(spec.Members, source.Members...)
	spec.Facets = append(spec.Facets, source.Facets...)
	return spec, nil
}

func (r *docResolver) validateSimpleContentInlineRestriction(base TypeRef, inline TypeRef) error {
	if base.IsZero() || inline.IsZero() {
		return nil
	}
	if r.typeRefDerivesFrom(inline, base, make(map[TypeRef]bool)) {
		return nil
	}
	return fmt.Errorf("schema ir: simpleContent nested simpleType %s does not derive from %s", formatName(inline.TypeName()), formatName(base.TypeName()))
}

func (r *docResolver) typeRefDerivesFrom(ref, target TypeRef, seen map[TypeRef]bool) bool {
	if ref == target {
		return true
	}
	if seen[ref] {
		return false
	}
	seen[ref] = true
	spec, ok := r.specForRef(ref)
	if !ok || spec.Base.IsZero() {
		return false
	}
	return r.typeRefDerivesFrom(spec.Base, target, seen)
}

func (r *docResolver) simpleContentBaseText(
	baseRef TypeRef,
	basePlan ComplexTypePlan,
	hasBasePlan bool,
) (TypeRef, SimpleTypeSpec, error) {
	if hasBasePlan {
		if !basePlan.TextType.IsZero() {
			return basePlan.TextType, SimpleTypeSpec{}, nil
		}
		if !isZeroSimpleTypeSpec(basePlan.TextSpec) {
			return NoTypeRef(), basePlan.TextSpec, nil
		}
	}
	if baseRef.IsZero() {
		return NoTypeRef(), SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent base missing")
	}
	if baseRef.IsBuiltin() {
		return baseRef, SimpleTypeSpec{}, nil
	}
	if decl := r.simpleDecls[r.ids.simpleByID[baseRef.TypeID()]]; decl != nil {
		if _, err := r.ensureSimpleType(decl, !decl.Name.IsZero()); err != nil {
			return NoTypeRef(), SimpleTypeSpec{}, err
		}
		return baseRef, SimpleTypeSpec{}, nil
	}
	return NoTypeRef(), SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent base %s is not simple", formatName(baseRef.TypeName()))
}

func (r *docResolver) simpleContentBaseSpec(ref TypeRef, spec SimpleTypeSpec) (SimpleTypeSpec, bool, error) {
	if !isZeroSimpleTypeSpec(spec) {
		return spec, true, nil
	}
	if ref.IsZero() {
		return SimpleTypeSpec{}, false, nil
	}
	if !ref.IsBuiltin() {
		if decl := r.simpleDecls[r.ids.simpleByID[ref.TypeID()]]; decl != nil {
			if _, err := r.ensureSimpleType(decl, !decl.Name.IsZero()); err != nil {
				return SimpleTypeSpec{}, false, err
			}
		}
	}
	spec, ok := r.specForRef(ref)
	return spec, ok, nil
}

func isZeroSimpleTypeSpec(spec SimpleTypeSpec) bool {
	return spec.TypeDecl == 0 &&
		spec.Name.Local == "" &&
		spec.Name.Namespace == "" &&
		!spec.Builtin &&
		spec.Primitive == "" &&
		spec.BuiltinBase == "" &&
		spec.Variety == TypeVarietyAtomic &&
		spec.Base.IsZero() &&
		spec.Item.IsZero() &&
		len(spec.Members) == 0 &&
		len(spec.Facets) == 0
}

func (r *docResolver) isIDType(ref TypeRef, seen map[TypeRef]bool) (bool, error) {
	if ref.IsBuiltin() {
		return ref.TypeName().Local == "ID", nil
	}
	if seen[ref] {
		return false, nil
	}
	seen[ref] = true
	if decl := r.simpleDecls[r.ids.simpleByID[ref.TypeID()]]; decl != nil {
		if _, err := r.ensureSimpleType(decl, !decl.Name.IsZero()); err != nil {
			return false, err
		}
	}
	spec, ok := r.specForRef(ref)
	if !ok {
		return false, nil
	}
	if spec.Name.Local == "ID" || spec.BuiltinBase == "ID" {
		return true, nil
	}
	if !spec.Base.IsZero() {
		return r.isIDType(spec.Base, seen)
	}
	return false, nil
}

func (r *docResolver) sortAttributeUses(ids []AttributeUseID) {
	slices.SortFunc(ids, func(a, b AttributeUseID) int {
		return compareName(r.attributeUseName(a), r.attributeUseName(b))
	})
}

func (r *docResolver) appendAttributeUses(base []AttributeUseID, ids []AttributeUseID, replaceByName bool) []AttributeUseID {
	for _, id := range ids {
		if replaceByName {
			name := r.attributeUseName(id)
			base = slices.DeleteFunc(base, func(existing AttributeUseID) bool {
				return r.attributeUseName(existing) == name
			})
		}
		base = append(base, id)
	}
	return base
}

func (r *docResolver) appendNonProhibitedAttributeUses(base []AttributeUseID, ids []AttributeUseID) []AttributeUseID {
	for _, id := range ids {
		if id == 0 || int(id) > len(r.out.AttributeUses) {
			continue
		}
		if r.out.AttributeUses[id-1].Use == AttributeProhibited {
			continue
		}
		base = append(base, id)
	}
	return base
}

func (r *docResolver) attributeUseName(id AttributeUseID) Name {
	if id == 0 || int(id) > len(r.out.AttributeUses) {
		return Name{}
	}
	return r.out.AttributeUses[id-1].Name
}

func (r *docResolver) ensureBaseComplexPlan(ref TypeRef) (ComplexTypePlan, bool, error) {
	refID := ref.TypeID()
	if ref.IsBuiltin() || refID == 0 {
		return ComplexTypePlan{}, false, nil
	}
	if r.emittingTypes[refID] {
		return ComplexTypePlan{}, false, nil
	}
	if decl := r.complexDecls[r.ids.complexByID[refID]]; decl != nil {
		if _, err := r.ensureComplexType(decl, !decl.Name.IsZero()); err != nil {
			return ComplexTypePlan{}, false, err
		}
	}
	for _, plan := range r.out.ComplexTypes {
		if plan.TypeDecl == refID {
			return plan, true, nil
		}
	}
	return ComplexTypePlan{}, false, nil
}

func isBuiltinAnyType(ref TypeRef) bool {
	name := ref.TypeName()
	return ref.IsBuiltin() && name.Namespace == ast.XSDNamespace && name.Local == "anyType"
}

func (r *docResolver) addAnyTypeContentParticle() ParticleID {
	wildcardID := r.addWildcard(&ast.WildcardDecl{
		Namespace:       ast.NSCAny,
		ProcessContents: ast.Lax,
		MinOccurs:       ast.OccursFromInt(0),
		MaxOccurs:       ast.OccursUnbounded,
	})
	id := ParticleID(len(r.out.Particles) + 1)
	r.out.Particles = append(r.out.Particles, WildcardParticle(id, wildcardID, Occurs{Value: 0}, Occurs{Unbounded: true}))
	return id
}

func (r *docResolver) addSequenceParticle(left, right ParticleID) ParticleID {
	id := ParticleID(len(r.out.Particles) + 1)
	r.out.Particles = append(r.out.Particles, GroupParticle(id, GroupSequence, []ParticleID{left, right}, Occurs{Value: 1}, Occurs{Value: 1}))
	return id
}

func (r *docResolver) intersectLocalWildcards(current WildcardID, hasCurrent bool, next WildcardID) (WildcardID, bool) {
	if !hasCurrent {
		return next, true
	}
	return r.intersectWildcards(current, next), true
}

func (r *docResolver) unionWildcards(a, b WildcardID) (WildcardID, error) {
	if a == 0 {
		return b, nil
	}
	if b == 0 {
		return a, nil
	}
	merged := ast.UnionAnyAttribute(r.anyAttributeFromWildcard(a), r.anyAttributeFromWildcard(b))
	if merged == nil {
		return 0, fmt.Errorf("schema ir: anyAttribute derivation: anyAttribute extension: union of derived and base anyAttribute is not expressible")
	}
	return r.addAnyAttributeWildcard(merged), nil
}

func (r *docResolver) cloneWildcard(id WildcardID) WildcardID {
	if id == 0 || int(id) > len(r.out.Wildcards) {
		return 0
	}
	wildcard := r.out.Wildcards[id-1]
	wildcard.ID = WildcardID(len(r.out.Wildcards) + 1)
	wildcard.Namespaces = slices.Clone(wildcard.Namespaces)
	r.out.Wildcards = append(r.out.Wildcards, wildcard)
	return wildcard.ID
}

func (r *docResolver) intersectWildcards(a, b WildcardID) WildcardID {
	if a == 0 || b == 0 {
		return 0
	}
	merged, expressible, empty := ast.IntersectAnyAttributeDetailed(
		r.anyAttributeFromWildcard(a),
		r.anyAttributeFromWildcard(b),
	)
	if !expressible || empty || merged == nil {
		return 0
	}
	return r.addAnyAttributeWildcard(merged)
}

func (r *docResolver) anyAttributeFromWildcard(id WildcardID) *ast.AnyAttribute {
	if id == 0 || int(id) > len(r.out.Wildcards) {
		return nil
	}
	wildcard := r.out.Wildcards[id-1]
	anyAttr := &ast.AnyAttribute{
		Namespace:       astNamespaceKind(wildcard.NamespaceKind),
		TargetNamespace: ast.NamespaceURI(wildcard.TargetNamespace),
		ProcessContents: astProcessContents(wildcard.ProcessContents),
	}
	for _, ns := range wildcard.Namespaces {
		anyAttr.NamespaceList = append(anyAttr.NamespaceList, ast.NamespaceURI(ns))
	}
	return anyAttr
}

func (r *docResolver) anyElementFromWildcard(id WildcardID) *ast.AnyElement {
	if id == 0 || int(id) > len(r.out.Wildcards) {
		return nil
	}
	wildcard := r.out.Wildcards[id-1]
	anyElem := &ast.AnyElement{
		Namespace:       astNamespaceKind(wildcard.NamespaceKind),
		TargetNamespace: ast.NamespaceURI(wildcard.TargetNamespace),
		ProcessContents: astProcessContents(wildcard.ProcessContents),
		MinOccurs:       ast.OccursFromInt(1),
		MaxOccurs:       ast.OccursFromInt(1),
	}
	for _, ns := range wildcard.Namespaces {
		anyElem.NamespaceList = append(anyElem.NamespaceList, ast.NamespaceURI(ns))
	}
	return anyElem
}

func (r *docResolver) addAnyAttributeWildcard(anyAttr *ast.AnyAttribute) WildcardID {
	if anyAttr == nil {
		return 0
	}
	id := WildcardID(len(r.out.Wildcards) + 1)
	wildcard := Wildcard{
		ID:              id,
		NamespaceKind:   namespaceKind(anyAttr.Namespace),
		TargetNamespace: string(anyAttr.TargetNamespace),
		ProcessContents: processContents(anyAttr.ProcessContents),
	}
	for _, ns := range anyAttr.NamespaceList {
		wildcard.Namespaces = append(wildcard.Namespaces, string(ns))
	}
	r.out.Wildcards = append(r.out.Wildcards, wildcard)
	return id
}

func astNamespaceKind(kind NamespaceConstraintKind) ast.NamespaceConstraint {
	switch kind {
	case NamespaceOther:
		return ast.NSCOther
	case NamespaceTarget:
		return ast.NSCTargetNamespace
	case NamespaceLocal:
		return ast.NSCLocal
	case NamespaceList:
		return ast.NSCList
	case NamespaceNotAbsent:
		return ast.NSCNotAbsent
	default:
		return ast.NSCAny
	}
}

func astProcessContents(process ProcessContents) ast.ProcessContents {
	switch process {
	case ProcessLax:
		return ast.Lax
	case ProcessSkip:
		return ast.Skip
	default:
		return ast.Strict
	}
}

func occurs(value ast.Occurs) Occurs {
	if value.IsUnbounded() {
		return Occurs{Unbounded: true}
	}
	n, ok := value.Int()
	if !ok || n < 0 {
		return Occurs{Unbounded: true}
	}
	return Occurs{Value: uint32(n)}
}

func elementBlock(set ast.DerivationSet) ElementBlock {
	var out ElementBlock
	if set.Has(ast.DerivationSubstitution) {
		out |= ElementBlockSubstitution
	}
	if set.Has(ast.DerivationExtension) {
		out |= ElementBlockExtension
	}
	if set.Has(ast.DerivationRestriction) {
		out |= ElementBlockRestriction
	}
	return out
}

func attributeUseKind(use ast.AttributeUse) AttributeUseKind {
	switch use {
	case ast.Required:
		return AttributeRequired
	case ast.Prohibited:
		return AttributeProhibited
	default:
		return AttributeOptional
	}
}

func namespaceKind(kind ast.NamespaceConstraint) NamespaceConstraintKind {
	switch kind {
	case ast.NSCOther:
		return NamespaceOther
	case ast.NSCTargetNamespace:
		return NamespaceTarget
	case ast.NSCLocal:
		return NamespaceLocal
	case ast.NSCList:
		return NamespaceList
	case ast.NSCNotAbsent:
		return NamespaceNotAbsent
	default:
		return NamespaceAny
	}
}

func processContents(process ast.ProcessContents) ProcessContents {
	switch process {
	case ast.Lax:
		return ProcessLax
	case ast.Skip:
		return ProcessSkip
	default:
		return ProcessStrict
	}
}

func processContentsName(process ast.ProcessContents) string {
	switch process {
	case ast.Lax:
		return "lax"
	case ast.Skip:
		return "skip"
	default:
		return "strict"
	}
}
