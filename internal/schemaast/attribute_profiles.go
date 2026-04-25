package schemaast

type attributeProfile struct {
	allowed map[string]bool
}

func newAttributeProfile(names ...string) attributeProfile {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	return attributeProfile{allowed: allowed}
}

func (p attributeProfile) allows(name string) bool {
	return p.allowed[name]
}

var (
	topLevelElementAttributeProfile = newAttributeProfile(
		"id",
		"name",
		"type",
		"default",
		"fixed",
		"nillable",
		"abstract",
		"block",
		"final",
		"substitutionGroup",
	)

	localElementAttributeProfile = newAttributeProfile(
		"id",
		"name",
		"type",
		"minOccurs",
		"maxOccurs",
		"default",
		"fixed",
		"nillable",
		"block",
		"form",
		"ref",
	)

	attributeDeclarationProfile = newAttributeProfile(
		"name",
		"ref",
		"type",
		"use",
		"default",
		"fixed",
		"form",
		"id",
	)

	identityConstraintAttributeProfile = newAttributeProfile(
		"id",
		"name",
	)

	keyrefAttributeProfile = newAttributeProfile(
		"id",
		"name",
		"refer",
	)

	importDirectiveAttributeProfile = newAttributeProfile(
		"id",
		"namespace",
		"schemaLocation",
	)

	includeDirectiveAttributeProfile = newAttributeProfile(
		"id",
		"schemaLocation",
	)
)
