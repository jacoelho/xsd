package valuebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

// Compile builds runtime validator artifacts from immutable schema IR.
func Compile(schema *schemair.Schema) (*Artifacts, error) {
	c, err := newArtifactCompiler(schema)
	if err != nil {
		return nil, err
	}
	if err := c.compileBuiltins(); err != nil {
		return nil, fmt.Errorf("runtime build: compile builtin validators: %w", err)
	}
	if err := c.compileSimpleTypes(); err != nil {
		return nil, fmt.Errorf("runtime build: compile type validators: %w", err)
	}
	if err := c.compileTextValidators(); err != nil {
		return nil, fmt.Errorf("runtime build: compile text validators: %w", err)
	}
	if err := c.compileDefaults(); err != nil {
		return nil, fmt.Errorf("runtime build: compile defaults: %w", err)
	}
	return c.finish(), nil
}

func (c *artifactCompiler) compileBuiltins() error {
	for _, builtin := range c.schema.BuiltinTypes {
		id, err := c.compileSpec(builtin.Value)
		if err != nil {
			return fmt.Errorf("%s: %w", builtin.Name.Local, err)
		}
		c.out.BuiltinValidators[builtin.Name.Local] = id
	}
	return nil
}

func (c *artifactCompiler) compileSimpleTypes() error {
	for _, spec := range c.schema.SimpleTypes {
		id, err := c.compileSpec(spec)
		if err != nil {
			return fmt.Errorf("%s: %w", formatName(spec.Name), err)
		}
		c.out.TypeValidators[spec.TypeDecl] = id
	}
	return nil
}

func (c *artifactCompiler) compileTextValidators() error {
	for _, plan := range c.schema.ComplexTypes {
		if plan.Content != schemair.ContentSimple {
			continue
		}
		var (
			id  runtime.ValidatorID
			err error
		)
		if isZeroRef(plan.TextType) {
			id, err = c.compileSpec(plan.TextSpec)
		} else {
			id, err = c.compileRef(plan.TextType)
		}
		if err != nil {
			return fmt.Errorf("complex type %d text: %w", plan.TypeDecl, err)
		}
		c.out.TextValidators[plan.TypeDecl] = id
	}
	return nil
}

func (c *artifactCompiler) compileSpec(spec schemair.SimpleTypeSpec) (runtime.ValidatorID, error) {
	key := specKey(spec)
	cacheable := key.builtin != "" || key.id != 0
	if cacheable {
		if id, ok := c.validators[key]; ok {
			return id, nil
		}
		if c.compiling[key] {
			return 0, fmt.Errorf("simple type cycle at %s", formatName(spec.Name))
		}
		c.compiling[key] = true
		defer delete(c.compiling, key)
	}

	facets, err := c.compileFacetProgram(spec)
	if err != nil {
		return 0, err
	}

	var id runtime.ValidatorID
	switch spec.Variety {
	case schemair.TypeVarietyList:
		item, err := c.compileRef(spec.Item)
		if err != nil {
			return 0, fmt.Errorf("list item: %w", err)
		}
		id = c.addListValidator(runtimeWhitespace(spec.Whitespace), facets, item)
	case schemair.TypeVarietyUnion:
		id, err = c.compileUnion(spec, facets)
		if err != nil {
			return 0, err
		}
	default:
		kind, err := validatorKind(spec)
		if err != nil {
			return 0, err
		}
		id = c.addAtomicValidator(
			kind,
			runtimeWhitespace(spec.Whitespace),
			facets,
			stringKindForBuiltin(specBuiltinName(spec)),
			integerKindForBuiltin(specBuiltinName(spec)),
		)
	}
	if cacheable {
		c.validators[key] = id
	}
	return id, nil
}

func (c *artifactCompiler) compileUnion(spec schemair.SimpleTypeSpec, facets runtime.FacetProgramRef) (runtime.ValidatorID, error) {
	if len(spec.Members) == 0 {
		return 0, fmt.Errorf("union has no member types")
	}
	memberIDs := make([]runtime.ValidatorID, 0, len(spec.Members))
	memberTypes := make([]runtime.TypeID, 0, len(spec.Members))
	for _, ref := range spec.Members {
		id, err := c.compileRef(ref)
		if err != nil {
			return 0, err
		}
		typeID, ok := c.runtimeTypeID(ref)
		if !ok {
			return 0, fmt.Errorf("union member %s type id not found", formatName(ref.Name))
		}
		memberIDs = append(memberIDs, id)
		memberTypes = append(memberTypes, typeID)
	}
	typeID, _ := c.runtimeTypeID(typeRefForSpec(spec))
	return c.addUnionValidator(runtimeWhitespace(spec.Whitespace), facets, memberIDs, memberTypes, formatName(spec.Name), typeID)
}

func (c *artifactCompiler) compileRef(ref schemair.TypeRef) (runtime.ValidatorID, error) {
	spec, ok := c.specForRef(ref)
	if !ok {
		return 0, fmt.Errorf("validator for type %s not found", formatName(ref.Name))
	}
	return c.compileSpec(spec)
}

func (c *artifactCompiler) specForRef(ref schemair.TypeRef) (schemair.SimpleTypeSpec, bool) {
	if ref.Builtin {
		spec, ok := c.builtinSpecs[ref.Name.Local]
		return spec, ok
	}
	spec, ok := c.simpleSpecs[ref.ID]
	return spec, ok
}

func (c *artifactCompiler) runtimeTypeID(ref schemair.TypeRef) (runtime.TypeID, bool) {
	if isZeroRef(ref) {
		return 0, false
	}
	if ref.Builtin {
		id := c.builtinRuntimeIDs[ref.Name.Local]
		return id, id != 0
	}
	if ref.ID == 0 {
		return 0, false
	}
	return runtime.TypeID(len(c.schema.BuiltinTypes)) + runtime.TypeID(ref.ID), true
}

func specKey(spec schemair.SimpleTypeSpec) typeKey {
	if spec.Builtin {
		return typeKey{builtin: spec.Name.Local}
	}
	return typeKey{id: spec.TypeDecl}
}

func typeRefForSpec(spec schemair.SimpleTypeSpec) schemair.TypeRef {
	return schemair.TypeRef{
		ID:      spec.TypeDecl,
		Name:    spec.Name,
		Builtin: spec.Builtin,
	}
}
