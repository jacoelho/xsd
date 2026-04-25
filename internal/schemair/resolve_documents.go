package schemair

import (
	"cmp"
	"fmt"
	"slices"

	ast "github.com/jacoelho/xsd/internal/schemaast"
)

type docResolver struct {
	docs []ast.SchemaDocument
	out  Schema

	names    map[Name]struct{}
	contexts map[ast.NamespaceContextID]ast.NamespaceContext

	builtins    map[Name]TypeRef
	types       map[Name]TypeID
	elements    map[Name]ElementID
	attrs       map[Name]AttributeID
	groups      map[Name]*ast.GroupDecl
	attrgrps    map[Name]*ast.AttributeGroupDecl
	notations   map[Name]struct{}
	globalElems map[Name]*ast.ElementDecl
	globalAttrs map[Name]*ast.AttributeDecl

	simpleDecls    map[simpleDeclID]*ast.SimpleTypeDecl
	complexDecls   map[complexDeclID]*ast.ComplexTypeDecl
	elementDecls   map[elementDeclID]*ast.ElementDecl
	attributeDecls map[attributeDeclID]*ast.AttributeDecl

	simpleIDs   map[simpleDeclID]TypeID
	simpleByID  map[TypeID]simpleDeclID
	complexIDs  map[complexDeclID]TypeID
	complexByID map[TypeID]complexDeclID
	elemByID    map[ElementID]elementDeclID
	attrByID    map[AttributeID]attributeDeclID
	localElems  map[elementDeclID]ElementID
	localAttrs  map[attributeDeclID]AttributeID

	assignedSimpleTrees   map[simpleDeclID]bool
	assigningSimpleTrees  map[simpleDeclID]bool
	assignedComplexTrees  map[complexDeclID]bool
	assigningComplexTrees map[complexDeclID]bool
	emittedTypes          map[TypeID]bool
	emittingTypes         map[TypeID]bool
	emittedComplex        map[TypeID]bool
	emittedElems          map[ElementID]bool
	emittedAttrs          map[AttributeID]bool
	identityNames         map[Name]IdentityID
	identityKeys          map[Name]IdentityID
	particleChecks        []pendingParticleRestriction
	nextType              TypeID
	nextElem              ElementID
	nextAttr              AttributeID
}

type pendingParticleRestriction struct {
	base        TypeRef
	restriction ParticleID
}

type simpleDeclID uint32
type complexDeclID uint32
type elementDeclID uint32
type attributeDeclID uint32

func (r *docResolver) simpleDeclHandle(decl *ast.SimpleTypeDecl) simpleDeclID {
	if decl == nil {
		return 0
	}
	for id, existing := range r.simpleDecls {
		if existing == decl {
			return id
		}
	}
	id := simpleDeclID(len(r.simpleDecls) + 1)
	r.simpleDecls[id] = decl
	return id
}

func (r *docResolver) complexDeclHandle(decl *ast.ComplexTypeDecl) complexDeclID {
	if decl == nil {
		return 0
	}
	for id, existing := range r.complexDecls {
		if existing == decl {
			return id
		}
	}
	id := complexDeclID(len(r.complexDecls) + 1)
	r.complexDecls[id] = decl
	return id
}

func (r *docResolver) elementDeclHandle(decl *ast.ElementDecl) elementDeclID {
	if decl == nil {
		return 0
	}
	for id, existing := range r.elementDecls {
		if existing == decl {
			return id
		}
	}
	id := elementDeclID(len(r.elementDecls) + 1)
	r.elementDecls[id] = decl
	return id
}

func (r *docResolver) attributeDeclHandle(decl *ast.AttributeDecl) attributeDeclID {
	if decl == nil {
		return 0
	}
	for id, existing := range r.attributeDecls {
		if existing == decl {
			return id
		}
	}
	id := attributeDeclID(len(r.attributeDecls) + 1)
	r.attributeDecls[id] = decl
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
	r := &docResolver{
		docs:                  normalizedDocs,
		names:                 make(map[Name]struct{}),
		contexts:              contexts,
		builtins:              make(map[Name]TypeRef),
		types:                 make(map[Name]TypeID),
		elements:              make(map[Name]ElementID),
		attrs:                 make(map[Name]AttributeID),
		groups:                make(map[Name]*ast.GroupDecl),
		attrgrps:              make(map[Name]*ast.AttributeGroupDecl),
		notations:             make(map[Name]struct{}),
		globalElems:           make(map[Name]*ast.ElementDecl),
		globalAttrs:           make(map[Name]*ast.AttributeDecl),
		simpleDecls:           make(map[simpleDeclID]*ast.SimpleTypeDecl),
		complexDecls:          make(map[complexDeclID]*ast.ComplexTypeDecl),
		elementDecls:          make(map[elementDeclID]*ast.ElementDecl),
		attributeDecls:        make(map[attributeDeclID]*ast.AttributeDecl),
		simpleIDs:             make(map[simpleDeclID]TypeID),
		simpleByID:            make(map[TypeID]simpleDeclID),
		complexIDs:            make(map[complexDeclID]TypeID),
		complexByID:           make(map[TypeID]complexDeclID),
		elemByID:              make(map[ElementID]elementDeclID),
		attrByID:              make(map[AttributeID]attributeDeclID),
		localElems:            make(map[elementDeclID]ElementID),
		localAttrs:            make(map[attributeDeclID]AttributeID),
		assignedSimpleTrees:   make(map[simpleDeclID]bool),
		assigningSimpleTrees:  make(map[simpleDeclID]bool),
		assignedComplexTrees:  make(map[complexDeclID]bool),
		assigningComplexTrees: make(map[complexDeclID]bool),
		emittedTypes:          make(map[TypeID]bool),
		emittingTypes:         make(map[TypeID]bool),
		emittedComplex:        make(map[TypeID]bool),
		emittedElems:          make(map[ElementID]bool),
		emittedAttrs:          make(map[AttributeID]bool),
		identityNames:         make(map[Name]IdentityID),
		identityKeys:          make(map[Name]IdentityID),
		nextType:              1,
		nextElem:              1,
		nextAttr:              1,
	}
	if err := r.resolve(); err != nil {
		return nil, err
	}
	return &r.out, nil
}

func (r *docResolver) resolve() error {
	r.emitBuiltins()
	if err := r.validateImportVisibility(); err != nil {
		return err
	}
	if err := r.indexGlobals(); err != nil {
		return err
	}
	r.assignDeclarationIDs()
	if err := r.validateTypeDerivationCycles(); err != nil {
		return err
	}
	if err := r.emitGlobalDeclarations(); err != nil {
		return err
	}
	if err := r.validatePendingParticleRestrictions(); err != nil {
		return err
	}
	r.sortComponents()
	if err := r.validateSubstitutionCycles(); err != nil {
		return err
	}
	if err := r.resolveIdentityReferences(); err != nil {
		return err
	}
	r.emitReferences()
	r.emitGlobalIndexes()
	r.emitRuntimeNamePlan()
	r.emitNames()
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

func (r *docResolver) validateSubstitutionCycles() error {
	elements := make(map[ElementID]Element, len(r.out.Elements))
	starts := make([]ElementID, 0, len(r.out.Elements))
	for _, elem := range r.out.Elements {
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
		ref := TypeRef{Name: name, Builtin: true}
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

func (r *docResolver) indexGlobals() error {
	for di := range r.docs {
		doc := &r.docs[di]
		for i := range doc.Decls {
			decl := &doc.Decls[i]
			switch decl.Kind {
			case ast.DeclSimpleType:
				if decl.SimpleType == nil {
					return fmt.Errorf("schema ir: simple type declaration %s is nil", formatName(nameFromQName(decl.Name)))
				}
				if err := r.assignGlobalSimple(decl.SimpleType); err != nil {
					return err
				}
			case ast.DeclComplexType:
				if decl.ComplexType == nil {
					return fmt.Errorf("schema ir: complex type declaration %s is nil", formatName(nameFromQName(decl.Name)))
				}
				if err := r.assignGlobalComplex(decl.ComplexType); err != nil {
					return err
				}
			case ast.DeclElement:
				if decl.Element == nil {
					return fmt.Errorf("schema ir: element declaration %s is nil", formatName(nameFromQName(decl.Name)))
				}
				if err := r.assignGlobalElement(decl.Element); err != nil {
					return err
				}
			case ast.DeclAttribute:
				if decl.Attribute == nil {
					return fmt.Errorf("schema ir: attribute declaration %s is nil", formatName(nameFromQName(decl.Name)))
				}
				if err := r.assignGlobalAttribute(decl.Attribute); err != nil {
					return err
				}
			case ast.DeclGroup:
				if decl.Group == nil {
					return fmt.Errorf("schema ir: group declaration %s is nil", formatName(nameFromQName(decl.Name)))
				}
				name := nameFromQName(decl.Group.Name)
				if _, exists := r.groups[name]; exists {
					return fmt.Errorf("schema ir: duplicate group %s", formatName(name))
				}
				r.groups[name] = decl.Group
			case ast.DeclAttributeGroup:
				if decl.AttributeGroup == nil {
					return fmt.Errorf("schema ir: attributeGroup declaration %s is nil", formatName(nameFromQName(decl.Name)))
				}
				name := nameFromQName(decl.AttributeGroup.Name)
				if _, exists := r.attrgrps[name]; exists {
					return fmt.Errorf("schema ir: duplicate attributeGroup %s", formatName(name))
				}
				r.attrgrps[name] = decl.AttributeGroup
			case ast.DeclNotation:
				if decl.Notation == nil {
					return fmt.Errorf("schema ir: notation declaration %s is nil", formatName(nameFromQName(decl.Name)))
				}
				name := nameFromQName(decl.Notation.Name)
				if docIsZeroName(name) {
					return fmt.Errorf("schema ir: notation missing name")
				}
				if _, exists := r.notations[name]; exists {
					return fmt.Errorf("schema ir: duplicate notation %s", formatName(name))
				}
				r.notations[name] = struct{}{}
			}
		}
	}
	return nil
}

func (r *docResolver) assignGlobalSimple(decl *ast.SimpleTypeDecl) error {
	name := nameFromQName(decl.Name)
	if docIsZeroName(name) {
		return fmt.Errorf("schema ir: global simple type missing name")
	}
	if _, exists := r.types[name]; exists {
		return fmt.Errorf("schema ir: duplicate type %s", formatName(name))
	}
	r.types[name] = 0
	return nil
}

func (r *docResolver) assignGlobalComplex(decl *ast.ComplexTypeDecl) error {
	name := nameFromQName(decl.Name)
	if docIsZeroName(name) {
		return fmt.Errorf("schema ir: global complex type missing name")
	}
	if _, exists := r.types[name]; exists {
		return fmt.Errorf("schema ir: duplicate type %s", formatName(name))
	}
	r.types[name] = 0
	return nil
}

func (r *docResolver) assignGlobalElement(decl *ast.ElementDecl) error {
	name := nameFromQName(decl.Name)
	if docIsZeroName(name) {
		return fmt.Errorf("schema ir: global element missing name")
	}
	if _, exists := r.globalElems[name]; exists {
		return fmt.Errorf("schema ir: duplicate element %s", formatName(name))
	}
	r.globalElems[name] = decl
	return nil
}

func (r *docResolver) assignGlobalAttribute(decl *ast.AttributeDecl) error {
	name := nameFromQName(decl.Name)
	if docIsZeroName(name) {
		return fmt.Errorf("schema ir: global attribute missing name")
	}
	if _, exists := r.globalAttrs[name]; exists {
		return fmt.Errorf("schema ir: duplicate attribute %s", formatName(name))
	}
	r.globalAttrs[name] = decl
	return nil
}

func (r *docResolver) assignDeclarationIDs() {
	for di := range r.docs {
		doc := &r.docs[di]
		for i := range doc.Decls {
			decl := &doc.Decls[i]
			switch decl.Kind {
			case ast.DeclSimpleType:
				r.assignSimpleTypeTree(decl.SimpleType, true)
			case ast.DeclComplexType:
				r.assignComplexTypeTree(decl.ComplexType, true)
			case ast.DeclElement:
				r.assignElementID(decl.Element, true)
				if decl.Element != nil {
					r.assignTypeUseInlineIDs(decl.Element.Type)
				}
			case ast.DeclAttribute:
				r.assignAttributeID(decl.Attribute, true)
				if decl.Attribute != nil {
					r.assignTypeUseInlineIDs(decl.Attribute.Type)
				}
			case ast.DeclGroup:
				if decl.Group != nil {
					r.assignParticleInlineIDs(decl.Group.Particle)
				}
			case ast.DeclAttributeGroup:
				if decl.AttributeGroup != nil {
					for i := range decl.AttributeGroup.Attributes {
						r.assignAttributeUseInlineIDs(&decl.AttributeGroup.Attributes[i], true)
					}
				}
			}
		}
	}
}

func (r *docResolver) assignSimpleTypeTree(decl *ast.SimpleTypeDecl, global bool) {
	if decl == nil {
		return
	}
	handle := r.simpleDeclHandle(decl)
	if r.assignedSimpleTrees[handle] || r.assigningSimpleTrees[handle] {
		return
	}
	r.assigningSimpleTrees[handle] = true
	defer delete(r.assigningSimpleTrees, handle)
	r.assignSimpleTypeID(decl, global)
	r.assignSimpleTypeTree(decl.InlineBase, false)
	r.assignSimpleTypeTree(decl.InlineItem, false)
	for i := range decl.InlineMembers {
		r.assignSimpleTypeTree(&decl.InlineMembers[i], false)
	}
	r.assignedSimpleTrees[handle] = true
}

func (r *docResolver) assignSimpleTypeID(decl *ast.SimpleTypeDecl, global bool) {
	if decl == nil {
		return
	}
	handle := r.simpleDeclHandle(decl)
	if _, exists := r.simpleIDs[handle]; exists {
		return
	}
	id := r.nextType
	r.nextType++
	r.simpleIDs[handle] = id
	r.simpleByID[id] = handle
	if global {
		r.types[nameFromQName(decl.Name)] = id
	}
}

func (r *docResolver) assignComplexTypeTree(decl *ast.ComplexTypeDecl, global bool) {
	if decl == nil {
		return
	}
	handle := r.complexDeclHandle(decl)
	if r.assignedComplexTrees[handle] || r.assigningComplexTrees[handle] {
		return
	}
	r.assigningComplexTrees[handle] = true
	defer delete(r.assigningComplexTrees, handle)
	r.assignComplexTypeID(decl, global)
	r.assignSimpleTypeTree(decl.SimpleType, false)
	r.assignParticleInlineIDs(decl.Particle)
	for i := range decl.Attributes {
		r.assignAttributeUseInlineIDs(&decl.Attributes[i], false)
	}
	r.assignedComplexTrees[handle] = true
}

func (r *docResolver) validateTypeDerivationCycles() error {
	state := make(map[TypeID]uint8, len(r.simpleByID)+len(r.complexByID))
	ids := make([]TypeID, 0, len(r.simpleByID)+len(r.complexByID))
	for id := range r.simpleByID {
		ids = append(ids, id)
	}
	for id := range r.complexByID {
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
			if ref.Builtin || ref.ID == 0 {
				continue
			}
			if err := visit(ref.ID); err != nil {
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
	if decl := r.simpleDecls[r.simpleByID[id]]; decl != nil {
		return r.simpleDerivationRefs(decl)
	}
	if decl := r.complexDecls[r.complexByID[id]]; decl != nil {
		if decl.Base.IsZero() {
			return nil
		}
		return []TypeRef{r.typeRefZero(decl.Base)}
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
			refs = append(refs, TypeRef{ID: r.simpleIDs[r.simpleDeclHandle(decl.InlineBase)], Name: nameFromQName(decl.InlineBase.Name)})
		}
	case ast.SimpleDerivationList:
		if !decl.ItemType.IsZero() {
			refs = append(refs, r.typeRefZero(decl.ItemType))
		}
		if decl.InlineItem != nil {
			refs = append(refs, TypeRef{ID: r.simpleIDs[r.simpleDeclHandle(decl.InlineItem)], Name: nameFromQName(decl.InlineItem.Name)})
		}
	case ast.SimpleDerivationUnion:
		for _, member := range decl.MemberTypes {
			refs = append(refs, r.typeRefZero(member))
		}
		for i := range decl.InlineMembers {
			member := &decl.InlineMembers[i]
			refs = append(refs, TypeRef{ID: r.simpleIDs[r.simpleDeclHandle(member)], Name: nameFromQName(member.Name)})
		}
	}
	return refs
}

func (r *docResolver) typeNameByID(id TypeID) Name {
	if decl := r.simpleDecls[r.simpleByID[id]]; decl != nil {
		return nameFromQName(decl.Name)
	}
	if decl := r.complexDecls[r.complexByID[id]]; decl != nil {
		return nameFromQName(decl.Name)
	}
	return Name{}
}

func (r *docResolver) assignComplexTypeID(decl *ast.ComplexTypeDecl, global bool) {
	if decl == nil {
		return
	}
	handle := r.complexDeclHandle(decl)
	if _, exists := r.complexIDs[handle]; exists {
		return
	}
	id := r.nextType
	r.nextType++
	r.complexIDs[handle] = id
	r.complexByID[id] = handle
	if global {
		r.types[nameFromQName(decl.Name)] = id
	}
}

func (r *docResolver) assignTypeUseInlineIDs(use ast.TypeUse) {
	r.assignSimpleTypeTree(use.Simple, false)
	r.assignComplexTypeTree(use.Complex, false)
}

func (r *docResolver) assignParticleInlineIDs(decl *ast.ParticleDecl) {
	r.assignParticleInlineIDsWithStack(decl, nil)
}

func (r *docResolver) assignParticleInlineIDsWithStack(decl *ast.ParticleDecl, stack []Name) {
	if decl == nil {
		return
	}
	if decl.Kind == ast.ParticleGroup {
		groupName := nameFromQName(decl.GroupRef)
		if slices.Contains(stack, groupName) {
			return
		}
		group := r.groups[groupName]
		if group != nil {
			r.assignParticleInlineIDsWithStack(group.Particle, append(stack, groupName))
		}
		return
	}
	if decl.Element != nil {
		r.assignElementID(decl.Element, false)
		r.assignTypeUseInlineIDs(decl.Element.Type)
	}
	for i := range decl.Children {
		r.assignParticleInlineIDsWithStack(&decl.Children[i], stack)
	}
}

func (r *docResolver) assignAttributeUseInlineIDs(use *ast.AttributeUseDecl, emitLocalDecl bool) {
	if use == nil || use.Attribute == nil {
		return
	}
	if emitLocalDecl {
		r.assignAttributeID(use.Attribute, false)
	}
	r.assignTypeUseInlineIDs(use.Attribute.Type)
}

func (r *docResolver) assignElementID(decl *ast.ElementDecl, global bool) {
	if decl == nil || !decl.Ref.IsZero() {
		return
	}
	handle := r.elementDeclHandle(decl)
	if _, exists := r.localElems[handle]; exists {
		return
	}
	id := r.nextElem
	r.nextElem++
	r.localElems[handle] = id
	r.elemByID[id] = handle
	if global {
		r.elements[nameFromQName(decl.Name)] = id
	}
}

func (r *docResolver) assignAttributeID(decl *ast.AttributeDecl, global bool) {
	if decl == nil || !decl.Ref.IsZero() {
		return
	}
	handle := r.attributeDeclHandle(decl)
	if _, exists := r.localAttrs[handle]; exists {
		return
	}
	id := r.nextAttr
	r.nextAttr++
	r.localAttrs[handle] = id
	r.attrByID[id] = handle
	if global {
		r.attrs[nameFromQName(decl.Name)] = id
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
		group, ok := r.groups[groupName]
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
		nested, ok := r.attrgrps[nestedName]
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
	if id, ok := r.simpleIDs[handle]; ok {
		if r.emittedTypes[id] {
			return id, nil
		}
		if r.emittingTypes[id] {
			return 0, fmt.Errorf("schema ir: type derivation cycle at %s", formatName(nameFromQName(decl.Name)))
		}
		if err := r.emitSimpleType(id, decl, global); err != nil {
			return 0, err
		}
		return id, nil
	}
	id := r.nextType
	r.nextType++
	r.simpleIDs[handle] = id
	r.simpleByID[id] = handle
	if err := r.emitSimpleType(id, decl, false); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *docResolver) ensureComplexType(decl *ast.ComplexTypeDecl, global bool) (TypeID, error) {
	if decl == nil {
		return 0, nil
	}
	handle := r.complexDeclHandle(decl)
	if id, ok := r.complexIDs[handle]; ok {
		if r.emittedTypes[id] {
			return id, nil
		}
		if r.emittingTypes[id] {
			return 0, fmt.Errorf("schema ir: type derivation cycle at %s", formatName(nameFromQName(decl.Name)))
		}
		if err := r.emitComplexType(id, decl, global); err != nil {
			return 0, err
		}
		return id, nil
	}
	id := r.nextType
	r.nextType++
	r.complexIDs[handle] = id
	r.complexByID[id] = handle
	if err := r.emitComplexType(id, decl, false); err != nil {
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
	if !base.Builtin && base.ID == id {
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
	if isZeroTypeRef(ref) || ref.Builtin || derivation == DerivationNone {
		return nil
	}
	baseDecl, ok, err := r.typeInfoForRef(ref)
	if err != nil || !ok {
		return err
	}
	if baseDecl.Final&derivation == 0 {
		return nil
	}
	return fmt.Errorf("schema ir: cannot %s type %s: base type is final for %s", action, formatName(ref.Name), derivationLabel(derivation))
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
		if !resolved.Builtin && resolved.ID == id {
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
		return fmt.Errorf("schema ir: complexContent extension cannot derive from simpleType '%s'", base.Name.Local)
	case ast.ComplexDerivationRestriction:
		return fmt.Errorf("schema ir: complexContent restriction cannot derive from simpleType '%s'", base.Name.Local)
	default:
		return nil
	}
}

func (r *docResolver) validateComplexDerivationFinal(decl *ast.ComplexTypeDecl, base TypeRef) error {
	if decl == nil || base.Builtin || base.ID == 0 {
		return nil
	}
	baseDecl := r.complexDecls[r.complexByID[base.ID]]
	if baseDecl == nil {
		return nil
	}
	switch decl.Derivation {
	case ast.ComplexDerivationExtension:
		if baseDecl.Final.Has(ast.DerivationExtension) {
			return fmt.Errorf("schema ir: cannot extend type %s: base type is final for extension", formatName(base.Name))
		}
	case ast.ComplexDerivationRestriction:
		if baseDecl.Final.Has(ast.DerivationRestriction) {
			return fmt.Errorf("schema ir: cannot restrict type %s: base type is final for restriction", formatName(base.Name))
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
	var inheritedAny WildcardID
	var restrictionAttrs []AttributeUseID
	baseRef := r.typeRefZero(decl.Base)
	var basePlan ComplexTypePlan
	var hasBasePlan bool
	if !isZeroTypeRef(baseRef) {
		var err error
		basePlan, hasBasePlan, err = r.ensureBaseComplexPlan(baseRef)
		if err != nil {
			return err
		}
		if isBuiltinAnyType(baseRef) && decl.Derivation == ast.ComplexDerivationExtension && decl.Content == ast.ComplexContentComplex {
			plan.Particle = r.addAnyTypeContentParticle()
			plan.Content = ContentElement
		}
		if hasBasePlan && decl.Derivation == ast.ComplexDerivationExtension {
			plan.Attrs = r.appendNonProhibitedAttributeUses(plan.Attrs, basePlan.Attrs)
			inheritedAny = basePlan.AnyAttr
		}
		if hasBasePlan && decl.Derivation == ast.ComplexDerivationRestriction {
			plan.Attrs = append(plan.Attrs, basePlan.Attrs...)
			inheritedAny = basePlan.AnyAttr
		}
		if hasBasePlan && decl.Derivation == ast.ComplexDerivationExtension &&
			(basePlan.Content != ContentSimple || decl.Content == ast.ComplexContentSimple) {
			plan.Particle = basePlan.Particle
			plan.Content = basePlan.Content
			plan.TextType = basePlan.TextType
			plan.TextSpec = basePlan.TextSpec
		}
		if hasBasePlan && decl.Derivation == ast.ComplexDerivationExtension && decl.Particle != nil && basePlan.Content == ContentSimple {
			return fmt.Errorf("schema ir: cannot extend simpleContent type %s with particles", formatName(baseRef.Name))
		}
		if hasBasePlan && decl.Content == ast.ComplexContentSimple && isZeroTypeRef(plan.TextType) {
			plan.TextType = basePlan.TextType
		}
		if hasBasePlan && decl.Content == ast.ComplexContentComplex {
			if err := r.validateMixedContentDerivation(decl, baseRef, basePlan); err != nil {
				return err
			}
			if decl.Derivation == ast.ComplexDerivationRestriction && basePlan.Content == ContentSimple {
				return fmt.Errorf("schema ir: complexContent restriction cannot derive from simpleContent type '%s'", baseRef.Name.Local)
			}
		}
	}
	for i := range decl.Attributes {
		ids, err := r.attributeUses(&decl.Attributes[i], nil, false)
		if err != nil {
			return err
		}
		if decl.Derivation == ast.ComplexDerivationRestriction && hasBasePlan {
			restrictionAttrs = append(restrictionAttrs, ids...)
		}
		plan.Attrs = r.appendAttributeUses(plan.Attrs, ids, decl.Derivation == ast.ComplexDerivationRestriction)
	}
	var localAny WildcardID
	var hasLocalAny bool
	for _, group := range decl.AttributeGroups {
		ids, err := r.attributeGroupUses(nameFromQName(group), nil)
		if err != nil {
			return err
		}
		if decl.Derivation == ast.ComplexDerivationRestriction && hasBasePlan {
			restrictionAttrs = append(restrictionAttrs, ids...)
		}
		plan.Attrs = r.appendAttributeUses(plan.Attrs, ids, decl.Derivation == ast.ComplexDerivationRestriction)
		groupWildcard, hasGroupWildcard, err := r.attributeGroupWildcard(nameFromQName(group), nil)
		if err != nil {
			return err
		}
		if hasGroupWildcard {
			localAny, hasLocalAny = r.intersectLocalWildcards(localAny, hasLocalAny, groupWildcard)
		}
	}
	if decl.AnyAttribute != nil {
		own := r.addWildcard(decl.AnyAttribute)
		localAny, hasLocalAny = r.intersectLocalWildcards(localAny, hasLocalAny, own)
	}
	if decl.Derivation == ast.ComplexDerivationRestriction && hasBasePlan {
		if err := r.validateAttributeRestriction(basePlan.Attrs, restrictionAttrs, inheritedAny, complexAttributeRestrictionContext(decl)); err != nil {
			return err
		}
	}
	switch decl.Derivation {
	case ast.ComplexDerivationExtension:
		if hasLocalAny {
			anyAttr, err := r.unionWildcards(localAny, inheritedAny)
			if err != nil {
				return err
			}
			plan.AnyAttr = anyAttr
		} else if inheritedAny != 0 {
			plan.AnyAttr = r.cloneWildcard(inheritedAny)
		} else {
			plan.AnyAttr = 0
		}
	case ast.ComplexDerivationRestriction:
		if hasLocalAny && localAny != 0 {
			if inheritedAny == 0 {
				return fmt.Errorf("schema ir: complex type %d restricts absent anyAttribute", id)
			}
			if err := r.validateAnyAttributeRestriction(inheritedAny, localAny); err != nil {
				return err
			}
			plan.AnyAttr = r.intersectWildcards(inheritedAny, localAny)
		}
	default:
		if hasLocalAny {
			plan.AnyAttr = localAny
		}
	}
	if decl.Content == ast.ComplexContentSimple {
		plan.Content = ContentSimple
		textType, textSpec, err := r.simpleContentText(decl, baseRef, basePlan, hasBasePlan)
		if err != nil {
			return err
		}
		plan.TextType = textType
		plan.TextSpec = textSpec
	} else if decl.Particle != nil {
		if decl.Derivation == ast.ComplexDerivationExtension && decl.Particle.Kind == ast.ParticleAll && hasBasePlan && basePlan.Content != ContentAll && basePlan.Particle != 0 {
			return fmt.Errorf("schema ir: cannot extend non-empty content model with all content model")
		}
		particle, err := r.addParticle(decl.Particle, nil)
		if err != nil {
			return err
		}
		if decl.Derivation == ast.ComplexDerivationRestriction && particle != 0 {
			if hasBasePlan && basePlan.Particle != 0 {
				if err := r.validateParticleRestriction(basePlan.Particle, particle); err != nil {
					return err
				}
			} else if !isZeroTypeRef(baseRef) && !baseRef.Builtin {
				r.particleChecks = append(r.particleChecks, pendingParticleRestriction{
					base:        baseRef,
					restriction: particle,
				})
			}
		}
		if decl.Derivation == ast.ComplexDerivationExtension && basePlan.Content == ContentAll && particle != 0 {
			count, err := r.activeParticleChildCount(basePlan.Particle)
			if err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("schema ir: cannot extend all content model with additional particles")
			}
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
	}
	r.sortAttributeUses(plan.Attrs)
	if err := r.validateComplexPlan(plan); err != nil {
		return err
	}
	r.out.ComplexTypes = append(r.out.ComplexTypes, plan)
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
			return fmt.Errorf("schema ir: mixed content derivation: cannot extend mixed content type '%s' to element-only content", baseRef.Name.Local)
		}
		if !baseMixed && derivedMixed {
			return fmt.Errorf("schema ir: mixed content derivation: cannot extend element-only content type '%s' to mixed content", baseRef.Name.Local)
		}
	case ast.ComplexDerivationRestriction:
		if !baseMixed && derivedMixed {
			return fmt.Errorf("schema ir: mixed content derivation: cannot restrict element-only content type '%s' to mixed content", baseRef.Name.Local)
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
		return TypeRef{}, SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent extension cannot have base type anyType")
	}
	baseTextRef, baseTextSpec, err := r.simpleContentBaseText(baseRef, basePlan, hasBasePlan)
	if err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}
	if decl.Derivation != ast.ComplexDerivationRestriction {
		return baseTextRef, baseTextSpec, nil
	}
	if err := r.validateSimpleContentRestrictionBase(baseRef); err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}

	baseSpec, ok, err := r.simpleContentBaseSpec(baseTextRef, baseTextSpec)
	if err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}
	if !ok {
		return TypeRef{}, SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent restriction base %s has no simple value type", formatName(baseRef.Name))
	}
	spec, err := r.simpleContentRestrictionSpec(decl, baseTextRef, baseSpec)
	if err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}

	baseWhitespace := spec.Whitespace
	ownWhitespace := false
	var ownFacets []FacetSpec
	for _, facet := range decl.SimpleFacets {
		if facet.Name == "whiteSpace" {
			spec.Whitespace = whitespaceModeFromString(facet.Lexical)
			ownWhitespace = true
			continue
		}
		converted, ok, err := r.facetSpec(facet)
		if err != nil {
			return TypeRef{}, SimpleTypeSpec{}, err
		}
		if ok {
			ownFacets = append(ownFacets, converted)
		}
	}
	ownFacets = coalesceFacetSpecs(ownFacets)
	if ownWhitespace && !validWhitespaceRestriction(baseWhitespace, spec.Whitespace) {
		return TypeRef{}, SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent restriction: whiteSpace facet value '%s' cannot be less restrictive than base type's '%s'", whitespaceModeString(spec.Whitespace), whitespaceModeString(baseWhitespace))
	}
	if len(ownFacets) > 0 && spec.Primitive == "anySimpleType" {
		return TypeRef{}, SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent restriction cannot apply facets to base type anySimpleType")
	}
	if err := validateFacetApplicability(spec, ownFacets); err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}
	if err := validateIRLengthFacetConsistency(spec, ownFacets); err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}
	if err := validateIRDigitsFacetConsistency(spec, ownFacets); err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}
	if err := validateRangeFacetConsistency(spec, ownFacets); err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}
	if err := validateIRFacetRestriction(spec, ownFacets); err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}
	if err := r.validateEnumerationLexicalValues(spec, ownFacets); err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}
	spec.Facets = append(spec.Facets, ownFacets...)
	if err := r.validateRestrictionEnumerations(spec); err != nil {
		return TypeRef{}, SimpleTypeSpec{}, err
	}
	if spec.Primitive == "" {
		spec.Primitive = fallbackSpecName(spec)
	}
	if spec.BuiltinBase == "" {
		spec.BuiltinBase = spec.Primitive
	}
	return TypeRef{}, spec, nil
}

func (r *docResolver) validateSimpleContentRestrictionBase(baseRef TypeRef) error {
	if isZeroTypeRef(baseRef) {
		return nil
	}
	if baseRef.Builtin {
		return fmt.Errorf("schema ir: simpleContent restriction cannot have simpleType base '%s'", formatName(baseRef.Name))
	}
	if r.simpleDecls[r.simpleByID[baseRef.ID]] != nil {
		return fmt.Errorf("schema ir: simpleContent restriction cannot have simpleType base '%s'", formatName(baseRef.Name))
	}
	return nil
}

func (r *docResolver) simpleContentRestrictionSpec(
	decl *ast.ComplexTypeDecl,
	baseTextRef TypeRef,
	baseSpec SimpleTypeSpec,
) (SimpleTypeSpec, error) {
	if decl.SimpleType != nil {
		inlineID, err := r.ensureSimpleType(decl.SimpleType, false)
		if err != nil {
			return SimpleTypeSpec{}, err
		}
		inlineRef := TypeRef{ID: inlineID, Name: nameFromQName(decl.SimpleType.Name)}
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
		inlineSpec.IntegerDerived = false
		inlineSpec.Whitespace = WhitespacePreserve
		r.setSimpleTypeSpec(inlineSpec)
	}
	spec := SimpleTypeSpec{
		Base:            baseTextRef,
		Variety:         baseSpec.Variety,
		Item:            baseSpec.Item,
		Primitive:       baseSpec.Primitive,
		BuiltinBase:     baseSpec.BuiltinBase,
		Whitespace:      baseSpec.Whitespace,
		QNameOrNotation: baseSpec.QNameOrNotation,
		IntegerDerived:  baseSpec.IntegerDerived,
	}
	spec.Members = append(spec.Members, baseSpec.Members...)
	spec.Facets = append(spec.Facets, baseSpec.Facets...)
	return spec, nil
}

func (r *docResolver) setSimpleTypeSpec(spec SimpleTypeSpec) {
	for i := range r.out.SimpleTypes {
		if r.out.SimpleTypes[i].TypeDecl == spec.TypeDecl {
			r.out.SimpleTypes[i] = spec
			return
		}
	}
}

func (r *docResolver) validateSimpleContentInlineRestriction(base TypeRef, inline TypeRef) error {
	if isZeroTypeRef(base) || isZeroTypeRef(inline) {
		return nil
	}
	if r.typeRefDerivesFrom(inline, base, make(map[TypeRef]bool)) {
		return nil
	}
	return fmt.Errorf("schema ir: simpleContent nested simpleType %s does not derive from %s", formatName(inline.Name), formatName(base.Name))
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
	if !ok || isZeroTypeRef(spec.Base) {
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
		if !isZeroTypeRef(basePlan.TextType) {
			return basePlan.TextType, SimpleTypeSpec{}, nil
		}
		if !isZeroSimpleTypeSpec(basePlan.TextSpec) {
			return TypeRef{}, basePlan.TextSpec, nil
		}
	}
	if isZeroTypeRef(baseRef) {
		return TypeRef{}, SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent base missing")
	}
	if baseRef.Builtin {
		return baseRef, SimpleTypeSpec{}, nil
	}
	if decl := r.simpleDecls[r.simpleByID[baseRef.ID]]; decl != nil {
		if _, err := r.ensureSimpleType(decl, !decl.Name.IsZero()); err != nil {
			return TypeRef{}, SimpleTypeSpec{}, err
		}
		return baseRef, SimpleTypeSpec{}, nil
	}
	return TypeRef{}, SimpleTypeSpec{}, fmt.Errorf("schema ir: simpleContent base %s is not simple", formatName(baseRef.Name))
}

func (r *docResolver) simpleContentBaseSpec(ref TypeRef, spec SimpleTypeSpec) (SimpleTypeSpec, bool, error) {
	if !isZeroSimpleTypeSpec(spec) {
		return spec, true, nil
	}
	if isZeroTypeRef(ref) {
		return SimpleTypeSpec{}, false, nil
	}
	if !ref.Builtin {
		if decl := r.simpleDecls[r.simpleByID[ref.ID]]; decl != nil {
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
		isZeroTypeRef(spec.Base) &&
		isZeroTypeRef(spec.Item) &&
		len(spec.Members) == 0 &&
		len(spec.Facets) == 0
}

func (r *docResolver) isIDType(ref TypeRef, seen map[TypeRef]bool) (bool, error) {
	if ref.Builtin {
		return ref.Name.Local == "ID", nil
	}
	if seen[ref] {
		return false, nil
	}
	seen[ref] = true
	if decl := r.simpleDecls[r.simpleByID[ref.ID]]; decl != nil {
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
	if !isZeroTypeRef(spec.Base) {
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
	if ref.Builtin || ref.ID == 0 {
		return ComplexTypePlan{}, false, nil
	}
	if r.emittingTypes[ref.ID] {
		return ComplexTypePlan{}, false, nil
	}
	if decl := r.complexDecls[r.complexByID[ref.ID]]; decl != nil {
		if _, err := r.ensureComplexType(decl, !decl.Name.IsZero()); err != nil {
			return ComplexTypePlan{}, false, err
		}
	}
	for _, plan := range r.out.ComplexTypes {
		if plan.TypeDecl == ref.ID {
			return plan, true, nil
		}
	}
	return ComplexTypePlan{}, false, nil
}

func isBuiltinAnyType(ref TypeRef) bool {
	return ref.Builtin && ref.Name.Namespace == ast.XSDNamespace && ref.Name.Local == "anyType"
}

func (r *docResolver) addAnyTypeContentParticle() ParticleID {
	wildcardID := r.addWildcard(&ast.WildcardDecl{
		Namespace:       ast.NSCAny,
		ProcessContents: ast.Lax,
		MinOccurs:       ast.OccursFromInt(0),
		MaxOccurs:       ast.OccursUnbounded,
	})
	id := ParticleID(len(r.out.Particles) + 1)
	r.out.Particles = append(r.out.Particles, Particle{
		ID:       id,
		Kind:     ParticleWildcard,
		Wildcard: wildcardID,
		Min:      Occurs{Value: 0},
		Max:      Occurs{Unbounded: true},
	})
	return id
}

func (r *docResolver) addSequenceParticle(left, right ParticleID) ParticleID {
	id := ParticleID(len(r.out.Particles) + 1)
	r.out.Particles = append(r.out.Particles, Particle{
		ID:       id,
		Kind:     ParticleGroup,
		Group:    GroupSequence,
		Children: []ParticleID{left, right},
		Min:      Occurs{Value: 1},
		Max:      Occurs{Value: 1},
	})
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
	any := &ast.AnyAttribute{
		Namespace:       astNamespaceKind(wildcard.NamespaceKind),
		TargetNamespace: ast.NamespaceURI(wildcard.TargetNamespace),
		ProcessContents: astProcessContents(wildcard.ProcessContents),
	}
	for _, ns := range wildcard.Namespaces {
		any.NamespaceList = append(any.NamespaceList, ast.NamespaceURI(ns))
	}
	return any
}

func (r *docResolver) anyElementFromWildcard(id WildcardID) *ast.AnyElement {
	if id == 0 || int(id) > len(r.out.Wildcards) {
		return nil
	}
	wildcard := r.out.Wildcards[id-1]
	any := &ast.AnyElement{
		Namespace:       astNamespaceKind(wildcard.NamespaceKind),
		TargetNamespace: ast.NamespaceURI(wildcard.TargetNamespace),
		ProcessContents: astProcessContents(wildcard.ProcessContents),
		MinOccurs:       ast.OccursFromInt(1),
		MaxOccurs:       ast.OccursFromInt(1),
	}
	for _, ns := range wildcard.Namespaces {
		any.NamespaceList = append(any.NamespaceList, ast.NamespaceURI(ns))
	}
	return any
}

func (r *docResolver) addAnyAttributeWildcard(any *ast.AnyAttribute) WildcardID {
	if any == nil {
		return 0
	}
	id := WildcardID(len(r.out.Wildcards) + 1)
	wildcard := Wildcard{
		ID:              id,
		NamespaceKind:   namespaceKind(any.Namespace),
		TargetNamespace: string(any.TargetNamespace),
		ProcessContents: processContents(any.ProcessContents),
	}
	for _, ns := range any.NamespaceList {
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
