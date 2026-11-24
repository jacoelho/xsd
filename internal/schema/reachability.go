package schema

import (
	"slices"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

// componentReach is the walk from the components an instance can name to the
// components validation can assess. An instance names only global
// declarations and types: a root element, xsi:type, a wildcard and a
// substitution group member all resolve by global name. From there validation
// reaches an element declaration's type, and a complex type's content model
// with every element particle in it and in its nested models, its text type
// and the types of its attribute uses. A simple type reaches its base, list
// item and union members, which its evaluation and diagnostics read. Each
// model, attribute-use set and simple type is visited once by ID, so a group
// or attribute group shared by many types is read once.
//
// The walk detaches every complex type before it starts and attaches each one
// it reaches, so complexTypeMetadata.Detached is its complex-type mark. The
// other marks share one allocation. Builtin simple types are always kept and
// name only builtins, so simpleTypes marks the user types, indexed from
// value.BuiltinTypeCount. A user simple type is walked depth-first: the value
// builder completed it only after its base, list item and union members, at a
// greater dependency depth, so the recursion is no deeper than
// maxComponentDependencyDepth.
type componentReach struct {
	elements      []bool
	models        []bool
	attributeSets []bool
	simpleTypes   []bool
	queue         []contentModelID
}

// detachUnreachableComponents marks the components no instance element
// reaches and returns the content models the walk reached, parallel to
// schemaBuild.Models. It is the one writer of complexTypeMetadata.Detached,
// identityConstraintSpec.Detached, schemaBuild.ReachableElements and
// schemaBuild.ReachableSimpleTypes. A
// constraint is detached when its declarer is unreached or it has none.
// Element declarations keep their identity lists, which the restriction
// checks compare; publication omits unreached declarations
// (publishedElements). Every element particle of a reached model names a
// reached declaration, so its compiled table can name declarations by their
// published IDs.
//
// The walk charges no budget. It reads the particles of each model once, and
// only models complexTypeReachableModels already read and charged under
// MaxContentModelAnalysisSteps, and it visits each element declaration and
// complex type once, from a global declaration or from such a particle, so
// its work is bounded by steps already spent. It reads the uses of each
// attribute-use set once, uses its compilation charged under
// MaxSchemaDependencySteps, and visits each simple type once, reading its
// base, list item and union members, which its definition charged when it
// was compiled.
func (c *compiler) detachUnreachableComponents() ([]bool, error) {
	r, err := c.reachFromGlobals()
	if err != nil {
		return nil, err
	}
	for i := range c.rt.Identities {
		declarer := c.rt.Identities[i].Declarer
		c.rt.Identities[i].Detached = declarer == NoElement || !validElementID(declarer, len(r.elements)) || !r.elements[declarer]
	}
	c.rt.ReachableElements = r.elements
	c.rt.ReachableSimpleTypes = r.simpleTypes
	return r.models, nil
}

func (c *compiler) reachFromGlobals() (componentReach, error) {
	userSimpleTypes := len(c.rt.SimpleTypes) - int(value.BuiltinTypeCount)
	if userSimpleTypes < 0 {
		return componentReach{}, xsderrors.InternalInvariant("simple type table does not hold the builtins")
	}
	for i := range c.rt.ComplexTypes {
		c.rt.ComplexTypes[i].Detached = true
	}
	marks := make([]bool, len(c.rt.Elements)+len(c.rt.Models)+len(c.rt.AttributeUseSets)+userSimpleTypes)
	take := func(n int) []bool {
		taken := marks[:n:n]
		marks = marks[n:]
		return taken
	}
	r := componentReach{
		elements:      take(len(c.rt.Elements)),
		models:        take(len(c.rt.Models)),
		attributeSets: take(len(c.rt.AttributeUseSets)),
		simpleTypes:   take(userSimpleTypes),
	}
	if err := c.reachGlobalSimpleTypes(&r); err != nil {
		return componentReach{}, err
	}
	if err := c.reachGlobalDeclarations(&r); err != nil {
		return componentReach{}, err
	}
	// The queue grows while it is read, so its length is re-read each step.
	for next := 0; next < len(r.queue); {
		id := r.queue[next]
		next++
		if err := c.reachParticleElements(&r, id); err != nil {
			return componentReach{}, err
		}
	}
	return r, nil
}

// reachGlobalSimpleTypes roots the simple types validation can name without
// an element: the global types that xsi:type names, and the types of the
// global and intrinsic xsi attribute declarations.
func (c *compiler) reachGlobalSimpleTypes(r *componentReach) error {
	for i := int(value.BuiltinTypeCount); i < len(c.rt.SimpleTypes); i++ {
		if c.rt.SimpleTypes[i].Scope != declarationScopeGlobal {
			continue
		}
		if err := c.reachSimpleType(r, SimpleTypeID(i)); err != nil {
			return err
		}
	}
	for i := range c.rt.Attributes {
		if err := c.reachSimpleType(r, c.rt.Attributes[i].Type); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) reachSimpleType(r *componentReach, id SimpleTypeID) error {
	if id == NoSimpleType || id < value.BuiltinTypeCount {
		return nil
	}
	if uint64(id-value.BuiltinTypeCount) >= uint64(len(r.simpleTypes)) {
		return xsderrors.InternalInvariant("simple type reference is invalid")
	}
	user := int(id - value.BuiltinTypeCount)
	if r.simpleTypes[user] {
		return nil
	}
	r.simpleTypes[user] = true
	spec := &c.rt.SimpleTypes[id].ValueSpec
	if err := c.reachSimpleType(r, spec.Base); err != nil {
		return err
	}
	if err := c.reachSimpleType(r, spec.ListItem); err != nil {
		return err
	}
	for _, member := range spec.Union {
		if err := c.reachSimpleType(r, member); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) reachGlobalDeclarations(r *componentReach) error {
	for i := range c.rt.ComplexTypes {
		if c.rt.ComplexTypes[i].Scope != declarationScopeGlobal {
			continue
		}
		if err := c.reachComplexType(r, ComplexTypeID(i)); err != nil {
			return err
		}
	}
	for i := range c.rt.Elements {
		if c.rt.Elements[i].Scope != declarationScopeGlobal {
			continue
		}
		if err := c.reachElement(r, ElementID(i)); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) reachElement(r *componentReach, id ElementID) error {
	if r.elements[id] {
		return nil
	}
	r.elements[id] = true
	typ := c.rt.Elements[id].Type
	if simple, ok := typ.Simple(); ok {
		return c.reachSimpleType(r, simple)
	}
	if complexType, ok := typ.Complex(); ok {
		return c.reachComplexType(r, complexType)
	}
	return nil
}

func (c *compiler) reachComplexType(r *componentReach, id ComplexTypeID) error {
	if !validComplexTypeID(id, len(c.rt.ComplexTypes)) {
		return xsderrors.InternalInvariant("element declaration references invalid complex type")
	}
	if !c.rt.ComplexTypes[id].Detached {
		return nil
	}
	c.rt.ComplexTypes[id].Detached = false
	typ := &c.rt.ComplexTypes[id]
	if err := c.reachSimpleType(r, typ.TextType); err != nil {
		return err
	}
	if err := c.reachAttributeUseSet(r, typ.Attrs); err != nil {
		return err
	}
	return r.reachModel(typ.Content)
}

func (c *compiler) reachAttributeUseSet(r *componentReach, id attributeUseSetID) error {
	if !validAttributeUseSetID(id, len(r.attributeSets)) {
		return xsderrors.InternalInvariant("complex type references invalid attribute use set")
	}
	if r.attributeSets[id] {
		return nil
	}
	r.attributeSets[id] = true
	for _, use := range c.rt.AttributeUseSets[id].Uses {
		if err := c.reachSimpleType(r, use.Type); err != nil {
			return err
		}
	}
	return nil
}

func (r *componentReach) reachModel(id contentModelID) error {
	if !validContentModelID(id, len(r.models)) {
		return xsderrors.InternalInvariant("content model reference is invalid")
	}
	if !r.models[id] {
		r.models[id] = true
		r.queue = append(r.queue, id)
	}
	return nil
}

func (c *compiler) reachParticleElements(r *componentReach, id contentModelID) error {
	particles, err := chargedParticles(uncharged, c.rt.Models[id])
	if err != nil {
		return err
	}
	for _, particle := range particles {
		switch particle.Kind { //nolint:exhaustive // a wildcard reaches only global declarations, which are roots.
		case particleElement:
			if !validElementID(particle.Element, len(r.elements)) {
				return xsderrors.InternalInvariant("particle references invalid element")
			}
			err = c.reachElement(r, particle.Element)
		case particleModel:
			err = r.reachModel(particle.Model)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// publishedElements numbers the reached element declarations densely in
// declaration order: the published element read table holds them at those
// IDs, and every published holder names them by those IDs. The numbering is
// monotone, so an order by element ID, such as the member tie-break of the
// substitution table, survives it. An unreached declaration is not published
// and has no published ID. When every declaration is reached the numbering is
// the identity and holds no tables.
type publishedElements struct {
	// table maps each build ID to its published ID, or to NoElement for an
	// unreached declaration, and then each published ID to its build ID; nil
	// for the identity.
	table        []ElementID
	count        int
	declarations int
}

// publishedElementID is the published ID of an element declaration
// (publishedElements). A compiled content table names declarations only by
// published IDs, which newCompiledParticleRead alone produces from build IDs.
type publishedElementID ElementID

const noPublishedElement = publishedElementID(NoElement)

// newPublishedElements numbers the declarations reachable marks among the
// declarations build IDs below declarations name.
func newPublishedElements(reachable []bool, declarations int) (publishedElements, error) {
	if len(reachable) != declarations {
		return publishedElements{}, xsderrors.InternalInvariant("element reachability does not cover the element declarations")
	}
	count := 0
	for _, reached := range reachable {
		if reached {
			count++
		}
	}
	if count == declarations {
		return identityElements(declarations), nil
	}
	table := make([]ElementID, declarations+count)
	next := declarations
	for i, reached := range reachable {
		table[i] = NoElement
		if reached {
			table[i] = ElementID(next - declarations) // #nosec G115 -- next-declarations is below count, an element-declaration count.
			table[next] = ElementID(i)                // #nosec G115 -- i indexes an element-declaration table.
			next++
		}
	}
	return publishedElements{table: table, count: count, declarations: declarations}, nil
}

// identityElements publishes each of declarations under its build ID.
func identityElements(declarations int) publishedElements {
	return publishedElements{count: declarations, declarations: declarations}
}

// identity reports whether p publishes every declaration under its build ID.
func (p publishedElements) identity() bool {
	return p.table == nil
}

// id returns the published ID of the declaration buildID names, and false
// when buildID names no declaration or an unreached one.
func (p publishedElements) id(buildID ElementID) (ElementID, bool) {
	if !validElementID(buildID, p.declarations) {
		return NoElement, false
	}
	if p.table == nil {
		return buildID, true
	}
	id := p.table[buildID]
	return id, id != NoElement
}

// buildID returns the build ID of the declaration published as id, and false
// when id names no published declaration.
func (p publishedElements) buildID(id publishedElementID) (ElementID, bool) {
	if !validElementID(ElementID(id), p.count) {
		return NoElement, false
	}
	if p.table == nil {
		return ElementID(id), true
	}
	return p.table[p.declarations+int(id)], true
}

// numbers reports whether p is the numbering newPublishedElements derives
// from reachable.
func (p publishedElements) numbers(reachable []bool) bool {
	if len(reachable) != p.declarations {
		return false
	}
	next := 0
	for i, reached := range reachable {
		id, ok := p.id(ElementID(i)) // #nosec G115 -- reachable is parallel to an element-declaration table.
		if ok != reached || ok && int(id) != next {
			return false
		}
		if reached {
			next++
		}
	}
	return next == p.count
}

// compiledNumbering returns the numbering build's compiled content tables
// were built over, once it is checked to be the numbering of
// build.ReachableElements, so every published holder names declarations by
// the IDs the content tables use.
func compiledNumbering(build *schemaBuild) (publishedElements, error) {
	if len(build.ReachableElements) != len(build.Elements) {
		return publishedElements{}, xsderrors.InternalInvariant("element reachability does not cover the element declarations")
	}
	elements := identityElements(len(build.Elements))
	if build.CompiledContent.Elements != nil {
		elements = *build.CompiledContent.Elements
	}
	if !elements.numbers(build.ReachableElements) {
		return publishedElements{}, xsderrors.InternalInvariant("compiled content tables number element declarations other than reachability does")
	}
	return elements, nil
}

// publishedElementMaps returns the global element map and the substitution
// table over published IDs. Every global declaration is reached, so each has
// one unless the build is corrupt.
func publishedElementMaps(build *schemaBuild, published publishedElements) (map[QName]ElementID, substitutionTable, error) {
	globals := make(map[QName]ElementID, len(build.GlobalElements))
	for name, id := range build.GlobalElements {
		publishedID, ok := published.id(id)
		if !ok {
			return nil, substitutionTable{}, xsderrors.InternalInvariant("global element declaration has no published ID")
		}
		globals[name] = publishedID
	}
	substitutions, err := build.Substitutions.published(published)
	if err != nil {
		return nil, substitutionTable{}, err
	}
	return globals, substitutions, nil
}

// validateReachableHolders checks that what validation can read names no
// detached component: no global type is detached and a reachable element
// declaration's type is not detached. publishedContentModels checks the
// elements the content tables name.
func validateReachableHolders(build *schemaBuild) error {
	if err := validateGlobalComplexTypesAttached(build.ComplexTypes); err != nil {
		return err
	}
	for i := range build.Elements {
		if err := validateReachableElementType(build, ElementID(i)); err != nil {
			return err
		}
	}
	return nil
}

func validateGlobalComplexTypesAttached(types []complexTypeMetadata) error {
	for _, typ := range types {
		if typ.Detached && typ.Scope == declarationScopeGlobal {
			return xsderrors.InternalInvariant("global complex type is detached")
		}
	}
	return nil
}

func validateReachableElementType(build *schemaBuild, id ElementID) error {
	typ, ok := build.Elements[id].Type.Complex()
	if build.ReachableElements[id] && ok && validComplexTypeID(typ, len(build.ComplexTypes)) && build.ComplexTypes[typ].Detached {
		return xsderrors.InternalInvariant("reachable element declaration has a detached complex type")
	}
	return nil
}

// validatePublishedIdentityConstraints checks that the published element
// reads name exactly the constraints that are not detached, each once, so
// every constraint with an evaluation program has an element that opens its
// scope and no element names a constraint without one.
func validatePublishedIdentityConstraints(elements *elementReadTable, identities []identityConstraintSpec) error {
	named := make([]bool, len(identities))
	for _, id := range elements.identities {
		if !validIdentityConstraintID(id, len(identities)) || named[id] {
			return xsderrors.InternalInvariant("published element reads name an identity constraint invalidly or twice")
		}
		named[id] = true
	}
	for id := range identities {
		if identities[id].Detached == named[id] {
			return xsderrors.InternalInvariant("published element reads disagree with identity constraint detachment")
		}
	}
	return nil
}

// validatePublishedSimpleTypes checks that every simple type a published read
// names is one build.ReachableSimpleTypes marks, so sealing the value program
// keeping only the reached types leaves no read naming a type without a
// definition. When every simple type is reached no read can name an unreached
// one, so the reads are not scanned.
func validatePublishedSimpleTypes(p *schemaProgram, build *schemaBuild) error {
	r := reachedSimpleTypes(build.ReachableSimpleTypes)
	switch {
	case len(r) != len(build.SimpleTypes)-int(value.BuiltinTypeCount):
		return xsderrors.InternalInvariant("reachable simple types do not cover the simple types")
	case !slices.Contains(r, false):
		return nil
	case !r.keepElementTypes(p.Elements.meta):
		return xsderrors.InternalInvariant("published element declaration names an unreached simple type")
	case !r.keepTextTypes(p.ComplexTypes):
		return xsderrors.InternalInvariant("published complex type names an unreached text type")
	case !r.keepAttributeUseTypes(p.AttributeUseSets):
		return xsderrors.InternalInvariant("published attribute use names an unreached simple type")
	case !r.keepAttributeDeclTypes(p.Attributes):
		return xsderrors.InternalInvariant("published attribute declaration names an unreached simple type")
	case !r.keepGlobalTypes(p.GlobalTypes):
		return xsderrors.InternalInvariant("global simple type is unreached")
	}
	return nil
}

// reachedSimpleTypes is schemaBuild.ReachableSimpleTypes.
type reachedSimpleTypes []bool

func (r reachedSimpleTypes) keeps(id SimpleTypeID) bool {
	if id == NoSimpleType || id < value.BuiltinTypeCount {
		return true
	}
	user := uint64(id - value.BuiltinTypeCount)
	return user < uint64(len(r)) && r[user]
}

func (r reachedSimpleTypes) keepsType(typ TypeID) bool {
	id, simple := typ.Simple()
	return !simple || r.keeps(id)
}

func (r reachedSimpleTypes) keepElementTypes(meta []elementReadMeta) bool {
	for i := range meta {
		if !r.keepsType(meta[i].typ) {
			return false
		}
	}
	return true
}

func (r reachedSimpleTypes) keepTextTypes(types []complexTypeRead) bool {
	for i := range types {
		if !r.keeps(types[i].textType) {
			return false
		}
	}
	return true
}

func (r reachedSimpleTypes) keepAttributeUseTypes(sets []AttributeUseSetRead) bool {
	for i := range sets {
		for _, use := range sets[i].uses {
			if !r.keeps(use.typ) {
				return false
			}
		}
	}
	return true
}

func (r reachedSimpleTypes) keepAttributeDeclTypes(decls []AttributeDeclRead) bool {
	for i := range decls {
		if !r.keeps(decls[i].typ) {
			return false
		}
	}
	return true
}

func (r reachedSimpleTypes) keepGlobalTypes(types map[QName]TypeID) bool {
	for _, typ := range types {
		if !r.keepsType(typ) {
			return false
		}
	}
	return true
}

func uncharged(int) error { return nil }
