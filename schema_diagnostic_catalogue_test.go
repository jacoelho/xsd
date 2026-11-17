package xsd_test

import "github.com/jacoelho/xsd/xsderrors"

// The catalogue of single schema-document violations. Each entry's tier and
// stage is transcribed from the schema document diagnostics contract in
// ARCHITECTURE.md (internal/schema), never from compiler output; its code and
// message fragment only identify which violation the compiler reported.

// The reasons a known gap stays open.
const (
	gapValueSyntax    = "the value syntax of this attribute is checked when its component compiles"
	gapSchemaDefaults = "the defaults of xs:schema are checked after the document is parsed"
	gapEmptyNamespace = "an empty namespace name is rejected after the document is parsed"
	gapDuplicateID    = "ids are compared after the document is parsed"
)

// knownSchemaDiagnosticOrderGaps names the catalogue entries the compiler
// decides later than the contract declares, with the reason each stays open.
var knownSchemaDiagnosticOrderGaps = map[string]string{
	"anonymous complex type/mixed of type is no boolean":               gapValueSyntax,
	"attribute reference/use is no use":                                gapValueSyntax,
	"content derivation/mixed of content is no boolean":                gapValueSyntax,
	"content derivation/use is no use":                                 gapValueSyntax,
	"element reference/maxOccurs of reference is no occurrence count":  gapValueSyntax,
	"element reference/minOccurs of reference is no occurrence count":  gapValueSyntax,
	"facet/final of type is no derivation set":                         gapValueSyntax,
	"facet/fixed of facet is no boolean":                               gapValueSyntax,
	"group reference/maxOccurs of reference is no occurrence count":    gapValueSyntax,
	"group reference/minOccurs of reference is no occurrence count":    gapValueSyntax,
	"identity constraint/nillable of declaration is no boolean":        gapValueSyntax,
	"import/namespace is empty":                                        gapEmptyNamespace,
	"local attribute/abstract of type is no boolean":                   gapValueSyntax,
	"local attribute/block of type is no derivation set":               gapValueSyntax,
	"local attribute/final of type is no derivation set":               gapValueSyntax,
	"local attribute/form is no form":                                  gapValueSyntax,
	"local attribute/mixed of type is no boolean":                      gapValueSyntax,
	"local attribute/use is no use":                                    gapValueSyntax,
	"local element/block is no derivation set":                         gapValueSyntax,
	"local element/form is no form":                                    gapValueSyntax,
	"local element/maxOccurs of declaration is no occurrence count":    gapValueSyntax,
	"local element/maxOccurs of group is no occurrence count":          gapValueSyntax,
	"local element/minOccurs of declaration is no occurrence count":    gapValueSyntax,
	"local element/minOccurs of group is no occurrence count":          gapValueSyntax,
	"local element/minOccurs of group is unbounded":                    gapValueSyntax,
	"local element/nillable of declaration is no boolean":              gapValueSyntax,
	"schema root/attributeFormDefault is no form":                      gapSchemaDefaults,
	"schema root/blockDefault is no derivation set":                    gapSchemaDefaults,
	"schema root/elementFormDefault is no form":                        gapSchemaDefaults,
	"schema root/finalDefault is no derivation set":                    gapSchemaDefaults,
	"schema root/targetNamespace is empty":                             gapEmptyNamespace,
	"simple content/fixed of facet is no boolean":                      gapValueSyntax,
	"standalone/id used twice":                                         gapDuplicateID,
	"top-level element/abstract of declaration is no boolean":          gapValueSyntax,
	"top-level element/block is no derivation set":                     gapValueSyntax,
	"top-level element/block names a derivation elements cannot block": gapValueSyntax,
	"top-level element/final combines #all with a derivation":          gapValueSyntax,
	"top-level element/final is no derivation set":                     gapValueSyntax,
	"top-level element/nillable of declaration is no boolean":          gapValueSyntax,
	"wildcard/maxOccurs of any is no occurrence count":                 gapValueSyntax,
	"wildcard/minOccurs of any is no occurrence count":                 gapValueSyntax,
	"wildcard/namespace of any is no namespace list":                   gapValueSyntax,
	"wildcard/namespace of anyAttribute is no namespace list":          gapValueSyntax,
	"wildcard/processContents of any is no mode":                       gapValueSyntax,
	"wildcard/processContents of anyAttribute is no mode":              gapValueSyntax,
}

// sameStartTagFixtures are the fixtures that must generate pairs of
// violations on one start tag.
var sameStartTagFixtures = []string{
	"schema root", "top-level element", "local element", "local attribute", "facet", "identity constraint",
	"group reference", "content derivation",
}

const (
	invalidAttribute = xsderrors.CodeSchemaInvalidAttribute
	contentModel     = xsderrors.CodeSchemaContentModel
	reference        = xsderrors.CodeSchemaReference
	identity         = xsderrors.CodeSchemaIdentity
	occurrence       = xsderrors.CodeSchemaOccurrence
	facetCode        = xsderrors.CodeSchemaFacet
	duplicate        = xsderrors.CodeSchemaDuplicate
	xsd11            = xsderrors.CodeUnsupportedXSD11
)

func rootViolation(name, message string) violation {
	return parseViolation(stageRoot, name, xsderrors.CodeSchemaRoot, message)
}

func vocabulary(name string, code xsderrors.Code, message string) violation {
	return parseViolation(stageVocabulary, name, code, message)
}

// earlierText is character data immediately before the element at key; the
// diagnostic names its parent, whose local name is parent.
func earlierText(key, parent string) violation {
	v := parseViolation(stageEarlierText, "text before "+key, contentModel, `xs:"`+parent+`" cannot contain text`).at(key)
	v.leadText = true
	return v
}

func childViolation(name string, code xsderrors.Code, message string) violation {
	return parseViolation(stageChild, name, code, message)
}

func attributeViolation(name string, code xsderrors.Code, message string) violation {
	return parseViolation(stageAttribute, name, code, message)
}

func unknownAttribute(key, local string) violation {
	return attributeViolation("unknown attribute on "+key, invalidAttribute, `"`+local+`" cannot have attribute "foo"`).at(key).setting(`foo="1"`)
}

func invalidID(key string) violation {
	return attributeViolation("id of "+key+" is no NCName", invalidAttribute, "schema id must be NCName").at(key).setting(`id="1x"`)
}

func unboundPrefix(key, attr string) violation {
	return attributeViolation("unbound prefix in "+attr+" of "+key, reference, `unbound QName prefix "p"`).at(key).setting(attr + `="p:N"`)
}

func invalidBoolean(key, attr string) violation {
	return attributeViolation(attr+" of "+key+" is no boolean", invalidAttribute, `invalid boolean attribute "`+attr+`"`).at(key).setting(attr + `="maybe"`)
}

func invalidOccurrence(key, attr string) violation {
	return attributeViolation(attr+" of "+key+" is no occurrence count", occurrence, "invalid "+attr+` "x"`).at(key).setting(attr + `="x"`)
}

func requiredAttribute(name string, code xsderrors.Code, message string) violation {
	return parseViolation(stageRequiredAttribute, name, code, message)
}

// textInside is character data after the last child of the element at key,
// whose local name is local.
func textInside(key, local string) violation {
	v := parseViolation(stageText, "text in "+key, contentModel, `xs:"`+local+`" cannot contain text`).at(key)
	v.tailText = true
	return v
}

func requiredChild(name string, code xsderrors.Code, message string) violation {
	return parseViolation(stageRequiredChild, name, code, message)
}

func schemaDiagnosticFixtures() []fixture {
	return []fixture{
		schemaRootFixture(),
		importFixture(),
		topLevelElementFixture(),
		localElementFixture(),
		elementReferenceFixture(),
		localAttributeFixture(),
		attributeReferenceFixture(),
		attributeGroupReferenceFixture(),
		groupReferenceFixture(),
		namedGroupFixture(),
		simpleTypeFixture(),
		listFixture(),
		unionFixture(),
		contentDerivationFixture(),
		simpleContentFixture(),
		identityConstraintFixture(),
		keyrefFixture(),
		wildcardFixture(),
		notationFixture(),
		anonymousComplexTypeFixture(),
		anonymousSimpleTypeFixture(),
		annotationFixture(),
		standaloneFixture(),
	}
}

func schemaRootFixture() fixture {
	const key = schemaRootKey
	return fixture{name: "schema root", violations: []violation{
		rootViolation("root is another XSD element", "root element must be xs:schema").at(key).renamed("xs:element"),
		rootViolation("root is an XSD 1.1 element", "root element must be xs:schema").at(key).renamed("xs:override"),
		rootViolation("root is a foreign element", "root element must be xs:schema").at(key).renamed("f:schema"),
		vocabulary("XSD 1.1 attribute defaultAttributes", xsd11, "XSD 1.1 attribute defaultAttributes on xs:schema").at(key).setting(`defaultAttributes="g"`),
		vocabulary("XSD 1.1 attribute xpathDefaultNamespace", xsd11, "XSD 1.1 attribute xpathDefaultNamespace on xs:schema").at(key).setting(`xpathDefaultNamespace="urn:x"`),
		unknownAttribute(key, "schema"),
		invalidID(key),
		attributeViolation("targetNamespace is no anyURI", invalidAttribute, `invalid "targetNamespace"`).at(key).setting(`targetNamespace="%zz"`),
		attributeViolation("targetNamespace is empty", invalidAttribute, "schema targetNamespace cannot be empty").at(key).setting(`targetNamespace=""`),
		attributeViolation("elementFormDefault is no form", invalidAttribute, `invalid "elementFormDefault" value "x"`).at(key).setting(`elementFormDefault="x"`),
		attributeViolation("attributeFormDefault is no form", invalidAttribute, `invalid "attributeFormDefault" value "x"`).at(key).setting(`attributeFormDefault="x"`),
		attributeViolation("blockDefault is no derivation set", invalidAttribute, `invalid schema blockDefault value "x"`).at(key).setting(`blockDefault="x"`),
		attributeViolation("finalDefault is no derivation set", invalidAttribute, `invalid schema finalDefault value "x"`).at(key).setting(`finalDefault="x"`),
		attributeViolation("XSD-namespace attribute", invalidAttribute, `schema namespace attribute "foo" is not allowed`).at(key).setting(`xs:foo="1"`),
		textInside(key, "schema"),
	}}
}

func importFixture() fixture {
	const key = "import"
	return fixture{
		name:   "import",
		prolog: true,
		build: func(n string) []*node {
			return []*node{xs("import", `namespace="urn:imported-`+n+`"`).as(key)}
		},
		violations: []violation{
			vocabulary("xs:redefine", xsderrors.CodeUnsupportedRedefine, "xs:redefine is not supported").at(key).renamed("xs:redefine"),
			vocabulary("XSD 1.1 element override", xsd11, "XSD 1.1 feature override is not supported").at(key).renamed("xs:override"),
			earlierText(key, "schema"),
			unknownAttribute(key, "import"),
			invalidID(key),
			attributeViolation("namespace is no anyURI", invalidAttribute, `invalid "namespace"`).at(key).setting(`namespace="%zz"`),
			attributeViolation("namespace is empty", invalidAttribute, "import namespace cannot be empty").at(key).setting(`namespace=""`),
			attributeViolation("schemaLocation is no anyURI", reference, `invalid "schemaLocation"`).at(key).setting(`schemaLocation="%zz"`),
			textInside(key, "import"),
			requiredAttribute("include without schemaLocation", reference, "include missing schemaLocation").at(key).
				alone(func(string) []*node { return []*node{xs("include").as(key)} }),
		},
	}
}

func topLevelElementFixture() fixture {
	const key = "declaration"
	return fixture{
		name: "top-level element",
		build: func(n string) []*node {
			return []*node{xs("element", `name="e`+n+`"`).as(key)}
		},
		violations: []violation{
			vocabulary("XSD 1.1 attribute targetNamespace", xsd11, "XSD 1.1 attribute targetNamespace on xs:element").at(key).setting(`targetNamespace="urn:x"`),
			vocabulary("XSD 1.1 element", xsd11, "XSD 1.1 feature defaultOpenContent is not supported").at(key).renamed("xs:defaultOpenContent"),
			earlierText(key, "schema"),
			childViolation("unknown XSD element", contentModel, `invalid top-level schema child "future"`).at(key).renamed("xs:future"),
			childViolation("model group at top level", contentModel, `invalid top-level schema child "sequence"`).at(key).renamed("xs:sequence"),
			childViolation("identity constraint child at top level", contentModel, "identity constraint must be inside element").at(key).renamed("xs:selector"),
			childViolation("foreign element at top level", contentModel, `foreign element "x" is not allowed in schema grammar`).at(key).renamed("f:x"),
			childViolation("xs:documentation outside xs:annotation", contentModel, `xs:"documentation" can appear only in xs:annotation`).at(key).renamed("xs:documentation"),
			unknownAttribute(key, "element"),
			invalidID(key),
			attributeViolation("name is no NCName", invalidAttribute, "schema component name must be NCName").at(key).setting(`name="1x"`),
			attributeViolation("minOccurs at top level", invalidAttribute, `top-level "element" cannot have "minOccurs"`).at(key).setting(`minOccurs="0"`),
			attributeViolation("ref at top level", invalidAttribute, `top-level "element" cannot have "ref"`).at(key).setting(`ref="other"`),
			attributeViolation("form at top level", invalidAttribute, `top-level "element" cannot have "form"`).at(key).setting(`form="qualified"`),
			attributeViolation("XSD-namespace attribute", invalidAttribute, `schema namespace attribute "foo" is not allowed`).at(key).setting(`xs:foo="1"`),
			attributeViolation("xml:base is no URI reference", reference, "invalid xml:base").at(key).setting(`xml:base="%zz"`),
			unboundPrefix(key, "type"),
			unboundPrefix(key, "substitutionGroup"),
			attributeViolation("type is no QName", reference, `invalid QName "1x"`).at(key).setting(`type="1x"`),
			invalidBoolean(key, "nillable"),
			invalidBoolean(key, "abstract"),
			attributeViolation("block is no derivation set", invalidAttribute, `invalid element block value "x"`).at(key).setting(`block="x"`),
			attributeViolation("final is no derivation set", invalidAttribute, `invalid element final value "x"`).at(key).setting(`final="x"`),
			attributeViolation("final combines #all with a derivation", invalidAttribute, "element final cannot combine #all with other values").at(key).setting(`final="#all extension"`),
			attributeViolation("block names a derivation elements cannot block", invalidAttribute, `element block cannot contain "list"`).at(key).setting(`block="list"`),
			requiredAttribute("no name", reference, `top-level "element" missing name`).at(key).dropping("name"),
			textInside(key, "element"),
			compileViolation("default with fixed", invalidAttribute, "default").at(key).setting(`default="a"`, `fixed="b"`),
			compileViolation("type in a namespace that is not imported", reference, "namespace is not imported").at(key).setting(`type="f:other"`),
			compileViolation("default outside the type's value space", facetCode, "invalid element default value").at(key).setting(`type="xs:int"`, `default="abc"`),
		},
	}
}

func localElementFixture() fixture {
	const key, group, owner = "declaration", "group", "type"
	return fixture{
		name: "local element",
		build: func(n string) []*node {
			return []*node{xs("complexType", `name="t`+n+`"`).as(owner).with(
				xs("sequence").as(group).with(xs("element", `name="e"`).as(key)))}
		},
		violations: []violation{
			vocabulary("XSD 1.1 attribute targetNamespace", xsd11, "XSD 1.1 attribute targetNamespace on xs:element").at(key).setting(`targetNamespace="urn:x"`),
			vocabulary("XSD 1.1 element assert", xsd11, "XSD 1.1 feature assert is not supported").at(key).renamed("xs:assert"),
			earlierText(key, "sequence"),
			childViolation("xs:all in a model group", contentModel, "xs:all cannot be nested in model groups").at(key).renamed("xs:all"),
			childViolation("attribute in a model group", contentModel, `invalid model group child "attribute"`).at(key).renamed("xs:attribute"),
			childViolation("foreign element in a model group", contentModel, `foreign element "x" is not allowed in schema grammar`).at(key).renamed("f:x"),
			unknownAttribute(key, "element"),
			invalidID(key),
			attributeViolation("name is no NCName", invalidAttribute, "schema component name must be NCName").at(key).setting(`name="1x"`),
			attributeViolation("abstract on a local element", invalidAttribute, "local element cannot have abstract").at(key).setting(`abstract="true"`),
			attributeViolation("final on a local element", invalidAttribute, "local element cannot have final").at(key).setting(`final="#all"`),
			attributeViolation("substitutionGroup on a local element", invalidAttribute, "local element cannot have substitutionGroup").at(key).setting(`substitutionGroup="head"`),
			unboundPrefix(key, "type"),
			invalidBoolean(key, "nillable"),
			invalidOccurrence(key, "minOccurs"),
			invalidOccurrence(key, "maxOccurs"),
			attributeViolation("form is no form", invalidAttribute, `invalid element form value "x"`).at(key).setting(`form="x"`),
			attributeViolation("block is no derivation set", invalidAttribute, `invalid element block value "x"`).at(key).setting(`block="x"`),
			requiredAttribute("no name and no ref", reference, "local element missing name or ref").at(key).dropping("name"),
			textInside(key, "element"),
			compileViolation("maxOccurs below minOccurs", occurrence, "maxOccurs is less than minOccurs").at(key).setting(`minOccurs="2"`, `maxOccurs="1"`),
			compileViolation("maxOccurs above the occurrence limit", xsderrors.CodeSchemaLimit, "maxOccurs exceeds").at(key).setting(`maxOccurs="99999999999"`),

			earlierText(group, "complexType"),
			unknownAttribute(group, "sequence"),
			invalidOccurrence(group, "minOccurs"),
			invalidOccurrence(group, "maxOccurs"),
			attributeViolation("minOccurs of group is unbounded", occurrence, `invalid minOccurs "unbounded"`).at(group).setting(`minOccurs="unbounded"`),
			textInside(group, "sequence"),
			childViolation("second model group", contentModel, "complexType model group is out of order").at(group).after(func() *node { return xs("choice") }),
		},
	}
}

func elementReferenceFixture() fixture {
	const key = "reference"
	return fixture{
		name: "element reference",
		build: func(n string) []*node {
			return []*node{
				xs("element", `name="e`+n+`"`),
				xs("complexType", `name="t`+n+`"`).with(xs("sequence").with(xs("element", `ref="e`+n+`"`).as(key))),
			}
		},
		violations: []violation{
			earlierText(key, "sequence"),
			unknownAttribute(key, "element"),
			attributeViolation("type on a reference", invalidAttribute, `element ref cannot have attribute "type"`).at(key).setting(`type="xs:string"`),
			attributeViolation("name on a reference", invalidAttribute, `element ref cannot have attribute "name"`).at(key).setting(`name="other"`),
			attributeViolation("nillable on a reference", invalidAttribute, `element ref cannot have attribute "nillable"`).at(key).setting(`nillable="true"`),
			unboundPrefix(key, "ref"),
			attributeViolation("ref is no QName", reference, `invalid QName "1x"`).at(key).setting(`ref="1x"`),
			invalidOccurrence(key, "minOccurs"),
			invalidOccurrence(key, "maxOccurs"),
			textInside(key, "element"),
			compileViolation("unresolved ref", reference, "unknown element").at(key).setting(`ref="missing"`),
		},
	}
}

func localAttributeFixture() fixture {
	const key, owner = "declaration", "type"
	return fixture{
		name: "local attribute",
		build: func(n string) []*node {
			return []*node{xs("complexType", `name="t`+n+`"`).as(owner).with(xs("attribute", `name="a"`).as(key))}
		},
		violations: []violation{
			vocabulary("XSD 1.1 attribute inheritable", xsd11, "XSD 1.1 attribute inheritable on xs:attribute").at(key).setting(`inheritable="true"`),
			vocabulary("XSD 1.1 attribute targetNamespace", xsd11, "XSD 1.1 attribute targetNamespace on xs:attribute").at(key).setting(`targetNamespace="urn:x"`),
			earlierText(key, "complexType"),
			childViolation("attribute after anyAttribute", contentModel, "complexType attribute is out of order").at(key).after(func() *node { return xs("anyAttribute") }),
			unknownAttribute(key, "attribute"),
			invalidID(key),
			attributeViolation("name is no NCName", invalidAttribute, "schema component name must be NCName").at(key).setting(`name="1x"`),
			unboundPrefix(key, "type"),
			attributeViolation("use is no use", invalidAttribute, "invalid attribute use x").at(key).setting(`use="x"`),
			attributeViolation("form is no form", invalidAttribute, `invalid attribute form "x"`).at(key).setting(`form="x"`),
			requiredAttribute("no name and no ref", reference, "attribute missing name or ref").at(key).dropping("name"),
			textInside(key, "attribute"),
			compileViolation("default with fixed", invalidAttribute, "default").at(key).setting(`default="a"`, `fixed="b"`),
			compileViolation("default on a required attribute", invalidAttribute, "default").at(key).setting(`use="required"`, `default="a"`),
			compileViolation("attribute named xmlns", invalidAttribute, "xmlns").at(key).setting(`name="xmlns"`),

			vocabulary("XSD 1.1 attribute defaultAttributesApply", xsd11, "XSD 1.1 attribute defaultAttributesApply on xs:complexType").at(owner).setting(`defaultAttributesApply="true"`),
			unknownAttribute(owner, "complexType"),
			attributeViolation("name of type is no NCName", invalidAttribute, "schema component name must be NCName").at(owner).setting(`name="1x"`),
			invalidBoolean(owner, "abstract"),
			invalidBoolean(owner, "mixed"),
			attributeViolation("block of type is no derivation set", invalidAttribute, `invalid complexType block value "x"`).at(owner).setting(`block="x"`),
			attributeViolation("final of type is no derivation set", invalidAttribute, `invalid complexType final value "x"`).at(owner).setting(`final="x"`),
			requiredAttribute("type has no name", reference, `top-level "complexType" missing name`).at(owner).dropping("name"),
			textInside(owner, "complexType"),
		},
	}
}

func attributeReferenceFixture() fixture {
	const key = "reference"
	return fixture{
		name: "attribute reference",
		build: func(n string) []*node {
			return []*node{
				xs("attribute", `name="a`+n+`"`),
				xs("complexType", `name="t`+n+`"`).with(xs("attribute", `ref="a`+n+`"`).as(key)),
			}
		},
		violations: []violation{
			unknownAttribute(key, "attribute"),
			attributeViolation("type on a reference", invalidAttribute, `attribute ref cannot have attribute "type"`).at(key).setting(`type="xs:string"`),
			attributeViolation("form on a reference", invalidAttribute, `attribute ref cannot have attribute "form"`).at(key).setting(`form="qualified"`),
			unboundPrefix(key, "ref"),
			attributeViolation("use is no use", invalidAttribute, "invalid attribute use x").at(key).setting(`use="x"`),
			compileViolation("unresolved ref", reference, "unknown attribute").at(key).setting(`ref="missing"`),
		},
	}
}

func attributeGroupReferenceFixture() fixture {
	const key, group = "reference", "group"
	return fixture{
		name: "attribute group reference",
		build: func(n string) []*node {
			return []*node{
				xs("attributeGroup", `name="ag`+n+`"`).as(group).with(xs("attribute", `name="a"`)),
				xs("complexType", `name="t`+n+`"`).with(xs("attributeGroup", `ref="ag`+n+`"`).as(key)),
			}
		},
		violations: []violation{
			earlierText(key, "complexType"),
			childViolation("attribute group after anyAttribute", contentModel, "complexType attribute is out of order").at(key).after(func() *node { return xs("anyAttribute") }),
			unknownAttribute(key, "attributeGroup"),
			attributeViolation("name on a reference", invalidAttribute, `attributeGroup use cannot have attribute "name"`).at(key).setting(`name="other"`),
			unboundPrefix(key, "ref"),
			requiredAttribute("no ref", reference, "attributeGroup use missing ref").at(key).dropping("ref"),
			textInside(key, "attributeGroup"),
			compileViolation("unresolved ref", reference, "unknown attribute group").at(key).setting(`ref="missing"`),

			attributeViolation("ref on a definition", invalidAttribute, `top-level "attributeGroup" cannot have "ref"`).at(group).setting(`ref="other"`),
			requiredAttribute("definition has no name", reference, `top-level "attributeGroup" missing name`).at(group).dropping("name"),
			textInside(group, "attributeGroup"),
		},
	}
}

func groupReferenceFixture() fixture {
	const key = "reference"
	return fixture{
		name: "group reference",
		build: func(n string) []*node {
			return []*node{
				xs("group", `name="g`+n+`"`).with(xs("sequence")),
				xs("complexType", `name="t`+n+`"`).with(xs("group", `ref="g`+n+`"`).as(key)),
			}
		},
		violations: []violation{
			earlierText(key, "complexType"),
			childViolation("group reference after attribute", contentModel, "complexType model group is out of order").at(key).after(func() *node { return xs("attribute", `name="a"`) }),
			unknownAttribute(key, "group"),
			invalidID(key),
			attributeViolation("name on a reference", invalidAttribute, `group cannot have attribute "name"`).at(key).setting(`name="other"`),
			unboundPrefix(key, "ref"),
			invalidOccurrence(key, "minOccurs"),
			invalidOccurrence(key, "maxOccurs"),
			requiredAttribute("no ref", reference, "group use missing ref").at(key).dropping("ref"),
			textInside(key, "group"),
			compileViolation("unresolved ref", reference, "unknown model group").at(key).setting(`ref="missing"`),
		},
	}
}

func namedGroupFixture() fixture {
	const group, model, particle = "group", "model", "particle"
	return fixture{
		name: "named group",
		build: func(n string) []*node {
			return []*node{xs("group", `name="g`+n+`"`).as(group).with(
				xs("sequence").as(model).with(xs("element", `name="e"`).as(particle)))}
		},
		violations: []violation{
			attributeViolation("minOccurs on a named group", invalidAttribute, `top-level "group" cannot have "minOccurs"`).at(group).setting(`minOccurs="0"`),
			attributeViolation("ref on a named group", invalidAttribute, `top-level "group" cannot have "ref"`).at(group).setting(`ref="other"`),
			requiredAttribute("no name", reference, `top-level "group" missing name`).at(group).dropping("name"),
			textInside(group, "group"),
			requiredChild("no model group", contentModel, "top-level group must contain exactly one model group").at(group).without(model),

			earlierText(model, "group"),
			childViolation("second model group", contentModel, "top-level group must contain exactly one model group").at(model).after(func() *node { return xs("choice") }),
			childViolation("group reference in a named group", contentModel, "top-level group cannot contain group ref").at(model).renamed("xs:group"),
			childViolation("element in a named group", contentModel, `invalid top-level group child "element"`).at(model).renamed("xs:element"),
			unknownAttribute(model, "sequence"),
			attributeViolation("minOccurs on the model of a named group", invalidAttribute, "top-level model group cannot have minOccurs").at(model).setting(`minOccurs="0"`),
			attributeViolation("maxOccurs on the model of a named group", invalidAttribute, "top-level model group cannot have maxOccurs").at(model).setting(`maxOccurs="1"`),
			textInside(model, "sequence"),

			unknownAttribute(particle, "element"),
			requiredAttribute("particle has no name and no ref", reference, "local element missing name or ref").at(particle).dropping("name"),
		},
	}
}

func simpleTypeFixture() fixture {
	const owner, note, derivation, facet = "type", "annotation", "restriction", "facet"
	return fixture{
		name: "facet",
		build: func(n string) []*node {
			return []*node{xs("simpleType", `name="s`+n+`"`).as(owner).with(
				xs("annotation").as(note),
				xs("restriction", `base="xs:string"`).as(derivation).with(xs("maxLength", `value="5"`).as(facet)))}
		},
		violations: []violation{
			unknownAttribute(owner, "simpleType"),
			attributeViolation("final of type is no derivation set", invalidAttribute, `invalid simpleType final value "x"`).at(owner).setting(`final="x"`),
			requiredAttribute("type has no name", reference, `top-level "simpleType" missing name`).at(owner).dropping("name"),
			textInside(owner, "simpleType"),
			requiredChild("no derivation", contentModel, "simpleType must contain one restriction, list, or union").at(owner).without(derivation),

			unknownAttribute(note, "annotation"),
			invalidID(note),
			textInside(note, "annotation"),

			earlierText(derivation, "simpleType"),
			childViolation("second derivation", contentModel, "simpleType can contain one restriction, list, or union").at(derivation).after(func() *node { return xs("list", `itemType="xs:int"`) }),
			childViolation("content in a simple type", contentModel, `invalid simpleType child "complexContent"`).at(derivation).renamed("xs:complexContent"),
			unknownAttribute(derivation, "restriction"),
			unboundPrefix(derivation, "base"),
			attributeViolation("base is no QName", reference, `invalid QName "1x"`).at(derivation).setting(`base="1x"`),
			textInside(derivation, "restriction"),
			requiredChild("no base and no simpleType", reference, "simple restriction missing base").at(derivation).dropping("base"),
			compileViolation("unresolved base", reference, "unknown").at(derivation).setting(`base="missing"`),

			vocabulary("XSD 1.1 facet explicitTimezone", xsd11, "XSD 1.1 feature explicitTimezone is not supported").at(facet).renamed("xs:explicitTimezone"),
			earlierText(facet, "restriction"),
			childViolation("attribute in a simple restriction", contentModel, `invalid restriction child "attribute"`).at(facet).renamed("xs:attribute"),
			childViolation("simpleType after a facet", contentModel, "restriction simpleType must precede facets").at(facet).renamed("xs:simpleType").
				after(func() *node { return xs("minLength", `value="1"`) }),
			unknownAttribute(facet, "maxLength"),
			invalidID(facet),
			invalidBoolean(facet, "fixed"),
			requiredAttribute("no value", facetCode, "maxLength missing value").at(facet).dropping("value"),
			textInside(facet, "maxLength"),
			compileViolation("value is no length", facetCode, "maxLength").at(facet).setting(`value="-1"`),
			compileViolation("facet the base type does not admit", facetCode, "is not allowed").at(facet).renamed("xs:totalDigits"),
		},
	}
}

func listFixture() fixture {
	const key = "list"
	return fixture{
		name: "list",
		build: func(n string) []*node {
			return []*node{xs("simpleType", `name="s`+n+`"`).with(xs("list", `itemType="xs:int"`).as(key))}
		},
		violations: []violation{
			unknownAttribute(key, "list"),
			unboundPrefix(key, "itemType"),
			textInside(key, "list"),
			requiredChild("no itemType and no simpleType", reference, "list missing item type").at(key).dropping("itemType"),
		},
	}
}

func unionFixture() fixture {
	const key = "union"
	return fixture{
		name: "union",
		build: func(n string) []*node {
			return []*node{xs("simpleType", `name="s`+n+`"`).with(xs("union", `memberTypes="xs:int xs:string"`).as(key))}
		},
		violations: []violation{
			unknownAttribute(key, "union"),
			attributeViolation("unbound prefix in the second member type", reference, `unbound QName prefix "p"`).at(key).setting(`memberTypes="xs:int p:N"`),
			textInside(key, "union"),
			requiredChild("no memberTypes and no simpleType", reference, "union missing member types").at(key).dropping("memberTypes"),
			requiredChild("empty memberTypes and no simpleType", reference, "union missing member types").at(key).setting(`memberTypes=""`),
		},
	}
}

func contentDerivationFixture() fixture {
	const content, derivation, use = "content", "derivation", "attribute"
	return fixture{
		name: "content derivation",
		build: func(n string) []*node {
			return []*node{
				xs("complexType", `name="b`+n+`"`),
				xs("complexType", `name="t`+n+`"`).with(xs("complexContent").as(content).with(
					xs("extension", `base="b`+n+`"`).as(derivation).with(xs("attribute", `name="a"`).as(use)))),
			}
		},
		violations: []violation{
			unknownAttribute(content, "complexContent"),
			invalidBoolean(content, "mixed"),
			textInside(content, "complexContent"),
			requiredChild("no derivation", contentModel, "complexContent missing extension or restriction").at(content).without(derivation),

			earlierText(derivation, "complexContent"),
			childViolation("second derivation", contentModel, "complexContent can contain one derivation").at(derivation).
				after(func() *node { return xs("restriction", `base="xs:anyType"`) }),
			childViolation("model group in complexContent", contentModel, `invalid complexContent child "sequence"`).at(derivation).renamed("xs:sequence"),
			unknownAttribute(derivation, "extension"),
			invalidID(derivation),
			unboundPrefix(derivation, "base"),
			requiredAttribute("no base", reference, "complexContent extension missing base").at(derivation).dropping("base"),
			textInside(derivation, "extension"),
			compileViolation("unresolved base", reference, "unknown").at(derivation).setting(`base="missing"`),

			earlierText(use, "extension"),
			unknownAttribute(use, "attribute"),
			attributeViolation("use is no use", invalidAttribute, "invalid attribute use x").at(use).setting(`use="x"`),
			requiredAttribute("attribute has no name and no ref", reference, "attribute missing name or ref").at(use).dropping("name"),
		},
	}
}

func simpleContentFixture() fixture {
	const content, derivation, facet = "content", "derivation", "facet"
	return fixture{
		name: "simple content",
		build: func(n string) []*node {
			return []*node{
				xs("complexType", `name="b`+n+`"`).with(xs("simpleContent").with(xs("extension", `base="xs:string"`))),
				xs("complexType", `name="t`+n+`"`).with(xs("simpleContent").as(content).with(
					xs("restriction", `base="b`+n+`"`).as(derivation).with(xs("maxLength", `value="5"`).as(facet)))),
			}
		},
		violations: []violation{
			unknownAttribute(content, "simpleContent"),
			textInside(content, "simpleContent"),
			requiredChild("no derivation", contentModel, "simpleContent missing extension or restriction").at(content).without(derivation),

			unboundPrefix(derivation, "base"),
			requiredAttribute("no base", reference, "simpleContent restriction missing base").at(derivation).dropping("base"),

			earlierText(facet, "restriction"),
			childViolation("facet after attribute", contentModel, "simpleContent facet is out of order").at(facet).after(func() *node { return xs("attribute", `name="a"`) }),
			unknownAttribute(facet, "maxLength"),
			invalidBoolean(facet, "fixed"),
			requiredAttribute("facet has no value", facetCode, "maxLength missing value").at(facet).dropping("value"),
		},
	}
}

func identityConstraintFixture() fixture {
	const owner, key, selector, field = "declaration", "key", "selector", "field"
	return fixture{
		name: "identity constraint",
		build: func(n string) []*node {
			return []*node{xs("element", `name="e`+n+`"`).as(owner).with(
				xs("complexType").with(xs("attribute", `name="a"`)),
				xs("key", `name="k`+n+`"`).as(key).with(
					xs("selector", `xpath="."`).as(selector),
					xs("field", `xpath="@a"`).as(field)))}
		},
		violations: []violation{
			unknownAttribute(owner, "element"),
			invalidBoolean(owner, "nillable"),
			requiredAttribute("element has no name", reference, `top-level "element" missing name`).at(owner).dropping("name"),
			textInside(owner, "element"),

			vocabulary("XSD 1.1 attribute ref", xsd11, "XSD 1.1 attribute ref on xs:key").at(key).setting(`ref="other"`),
			earlierText(key, "element"),
			unknownAttribute(key, "key"),
			attributeViolation("name is no NCName", invalidAttribute, "schema component name must be NCName").at(key).setting(`name="1x"`),
			requiredAttribute("no name", identity, "identity constraint missing name").at(key).dropping("name"),
			textInside(key, "key"),
			requiredChild("no field", identity, "identity constraint missing fields").at(key).without(field),
			requiredChild("no selector and no field", identity, "identity constraint missing selector").at(key).without(selector, field),

			vocabulary("XSD 1.1 attribute xpathDefaultNamespace", xsd11, "XSD 1.1 attribute xpathDefaultNamespace on xs:selector").at(selector).setting(`xpathDefaultNamespace="urn:x"`),
			earlierText(selector, "key"),
			childViolation("field before selector", contentModel, "identity constraint field requires selector").at(selector).renamed("xs:field"),
			childViolation("element in an identity constraint", contentModel, `invalid identity constraint child "element"`).at(selector).renamed("xs:element"),
			unknownAttribute(selector, "selector"),
			invalidID(selector),
			attributeViolation("selector xpath is empty", identity, `"selector" xpath is empty`).at(selector).setting(`xpath=" "`),
			requiredAttribute("selector has no xpath", identity, `"selector" missing xpath`).at(selector).dropping("xpath"),
			textInside(selector, "selector"),
			compileViolation("selector xpath is no selector path", identity, "XPath").at(selector).setting(`xpath="///"`),

			earlierText(field, "key"),
			childViolation("second selector", contentModel, "identity constraint can contain at most one selector").at(field).renamed("xs:selector"),
			unknownAttribute(field, "field"),
			attributeViolation("field xpath is empty", identity, `"field" xpath is empty`).at(field).setting(`xpath=""`),
			requiredAttribute("field has no xpath", identity, `"field" missing xpath`).at(field).dropping("xpath"),
			compileViolation("field xpath is no field path", identity, "XPath").at(field).setting(`xpath="a/@b/c"`),
		},
	}
}

func keyrefFixture() fixture {
	const key = "keyref"
	return fixture{
		name: "keyref",
		build: func(n string) []*node {
			return []*node{xs("element", `name="e`+n+`"`).with(
				xs("complexType").with(xs("attribute", `name="a"`)),
				xs("key", `name="k`+n+`"`).with(xs("selector", `xpath="."`), xs("field", `xpath="@a"`)),
				xs("keyref", `name="r`+n+`"`, `refer="k`+n+`"`).as(key).with(xs("selector", `xpath="."`), xs("field", `xpath="@a"`)))}
		},
		violations: []violation{
			vocabulary("XSD 1.1 attribute ref", xsd11, "XSD 1.1 attribute ref on xs:keyref").at(key).setting(`ref="other"`),
			unknownAttribute(key, "keyref"),
			unboundPrefix(key, "refer"),
			requiredAttribute("no name", identity, "identity constraint missing name").at(key).dropping("name"),
			requiredAttribute("no refer", identity, "keyref missing refer").at(key).dropping("refer"),
			compileViolation("unresolved refer", reference, "unknown keyref refer").at(key).setting(`refer="missing"`),
		},
	}
}

func wildcardFixture() fixture {
	const particle, attribute = "any", "anyAttribute"
	return fixture{
		name: "wildcard",
		build: func(n string) []*node {
			return []*node{xs("complexType", `name="t`+n+`"`).with(
				xs("sequence").with(xs("any").as(particle)),
				xs("anyAttribute").as(attribute))}
		},
		violations: []violation{
			vocabulary("XSD 1.1 attribute notQName", xsd11, "XSD 1.1 attribute notQName on xs:any").at(particle).setting(`notQName="##defined"`),
			vocabulary("XSD 1.1 attribute notNamespace", xsd11, "XSD 1.1 attribute notNamespace on xs:any").at(particle).setting(`notNamespace="urn:x"`),
			earlierText(particle, "sequence"),
			unknownAttribute(particle, "any"),
			attributeViolation("processContents of any is no mode", invalidAttribute, "invalid processContents").at(particle).setting(`processContents="x"`),
			attributeViolation("namespace of any is no namespace list", invalidAttribute, `invalid wildcard namespace "##bogus"`).at(particle).setting(`namespace="##bogus"`),
			invalidOccurrence(particle, "minOccurs"),
			invalidOccurrence(particle, "maxOccurs"),
			textInside(particle, "any"),

			vocabulary("XSD 1.1 attribute notNamespace on anyAttribute", xsd11, "XSD 1.1 attribute notNamespace on xs:anyAttribute").at(attribute).setting(`notNamespace="urn:x"`),
			childViolation("second anyAttribute", contentModel, "complexType can contain at most one anyAttribute").at(attribute).after(func() *node { return xs("anyAttribute") }),
			unknownAttribute(attribute, "anyAttribute"),
			attributeViolation("occurrence on anyAttribute", invalidAttribute, `"anyAttribute" cannot have attribute "minOccurs"`).at(attribute).setting(`minOccurs="0"`),
			attributeViolation("processContents of anyAttribute is no mode", invalidAttribute, "invalid processContents").at(attribute).setting(`processContents="x"`),
			attributeViolation("namespace of anyAttribute is no namespace list", invalidAttribute, `invalid wildcard namespace "##bogus"`).at(attribute).setting(`namespace="##bogus"`),
		},
	}
}

func notationFixture() fixture {
	const key = "notation"
	return fixture{
		name: "notation",
		build: func(n string) []*node {
			return []*node{xs("notation", `name="n`+n+`"`, `public="p"`).as(key)}
		},
		violations: []violation{
			earlierText(key, "schema"),
			unknownAttribute(key, "notation"),
			attributeViolation("name is no NCName", invalidAttribute, "schema component name must be NCName").at(key).setting(`name="1x"`),
			attributeViolation("system is no anyURI", invalidAttribute, `invalid "system"`).at(key).setting(`system="%zz"`),
			requiredAttribute("no name", invalidAttribute, "notation missing name").at(key).dropping("name"),
			requiredAttribute("no public and no system", invalidAttribute, "notation requires public or system").at(key).dropping("public"),
			textInside(key, "notation"),
		},
	}
}

func anonymousComplexTypeFixture() fixture {
	const owner, key = "declaration", "type"
	return fixture{
		name: "anonymous complex type",
		build: func(n string) []*node {
			return []*node{xs("element", `name="e`+n+`"`).as(owner).with(xs("complexType").as(key).with(xs("sequence")))}
		},
		violations: []violation{
			vocabulary("XSD 1.1 attribute defaultAttributesApply", xsd11, "XSD 1.1 attribute defaultAttributesApply on xs:complexType").at(key).setting(`defaultAttributesApply="true"`),
			earlierText(key, "element"),
			childViolation("second anonymous type", contentModel, "element can contain at most one anonymous type").at(key).
				after(func() *node { return xs("simpleType").with(xs("restriction", `base="xs:string"`)) }),
			childViolation("type attribute and anonymous type", invalidAttribute, "element cannot have both type and anonymous type").at(key).editing(owner).setting(`type="xs:string"`),
			childViolation("attribute in an element", contentModel, `invalid element child "attribute"`).at(key).renamed("xs:attribute"),
			unknownAttribute(key, "complexType"),
			attributeViolation("name on an anonymous type", invalidAttribute, "local complexType cannot have name").at(key).setting(`name="other"`),
			attributeViolation("abstract on an anonymous type", invalidAttribute, "local complexType cannot have abstract").at(key).setting(`abstract="true"`),
			attributeViolation("block on an anonymous type", invalidAttribute, "local complexType cannot have block").at(key).setting(`block="#all"`),
			attributeViolation("final on an anonymous type", invalidAttribute, "local complexType cannot have final").at(key).setting(`final="#all"`),
			invalidBoolean(key, "mixed"),
			textInside(key, "complexType"),

			unknownAttribute(owner, "element"),
			unboundPrefix(owner, "substitutionGroup"),
			textInside(owner, "element"),
		},
	}
}

func anonymousSimpleTypeFixture() fixture {
	const owner, key, derivation = "declaration", "type", "restriction"
	return fixture{
		name: "anonymous simple type",
		build: func(n string) []*node {
			return []*node{xs("attribute", `name="a`+n+`"`).as(owner).with(
				xs("simpleType").as(key).with(xs("restriction", `base="xs:string"`).as(derivation)))}
		},
		violations: []violation{
			earlierText(key, "attribute"),
			childViolation("second simpleType", contentModel, "attribute can contain at most one simpleType").at(key).
				after(func() *node { return xs("simpleType").with(xs("restriction", `base="xs:string"`)) }),
			childViolation("type attribute and simpleType", invalidAttribute, "attribute cannot have both type and simpleType").at(key).editing(owner).setting(`type="xs:string"`),
			childViolation("complexType in an attribute", contentModel, `invalid attribute child "complexType"`).at(key).renamed("xs:complexType"),
			unknownAttribute(key, "simpleType"),
			attributeViolation("name on an anonymous type", invalidAttribute, "local simpleType cannot have name").at(key).setting(`name="other"`),
			attributeViolation("final on an anonymous type", invalidAttribute, "local simpleType cannot have final").at(key).setting(`final="#all"`),
			textInside(key, "simpleType"),
			requiredChild("no derivation", contentModel, "simpleType must contain one restriction, list, or union").at(key).without(derivation),

			attributeViolation("use at top level", invalidAttribute, `top-level "attribute" cannot have "use"`).at(owner).setting(`use="optional"`),
			attributeViolation("form at top level", invalidAttribute, `top-level "attribute" cannot have "form"`).at(owner).setting(`form="qualified"`),
			requiredAttribute("attribute has no name", reference, `top-level "attribute" missing name`).at(owner).dropping("name"),
			textInside(owner, "attribute"),

			childViolation("base attribute and simpleType", contentModel, "restriction cannot have both base and simpleType").at("inner").
				alone(func(n string) []*node {
					return []*node{xs("simpleType", `name="s`+n+`"`).with(xs("restriction", `base="xs:string"`).with(
						xs("simpleType").as("inner").with(xs("restriction", `base="xs:string"`))))}
				}),
			childViolation("itemType attribute and simpleType", contentModel, "list cannot have both itemType and simpleType").at("inner").
				alone(func(n string) []*node {
					return []*node{xs("simpleType", `name="s`+n+`"`).with(xs("list", `itemType="xs:int"`).with(
						xs("simpleType").as("inner").with(xs("restriction", `base="xs:int"`))))}
				}),
		},
	}
}

func annotationFixture() fixture {
	const key, payload = "annotation", "documentation"
	return fixture{
		name: "annotation",
		build: func(string) []*node {
			return []*node{xs("annotation").as(key).with(xs("documentation", `source="urn:x"`).as(payload))}
		},
		violations: []violation{
			earlierText(key, "schema"),
			unknownAttribute(key, "annotation"),
			invalidID(key),
			textInside(key, "annotation"),

			earlierText(payload, "annotation"),
			childViolation("element in an annotation", contentModel, `xs:"annotation" can contain only xs:appinfo and xs:documentation, not xs:"element"`).at(payload).renamed("xs:element"),
			childViolation("foreign element in an annotation", contentModel, `foreign element "x" is not allowed in schema grammar`).at(payload).renamed("f:x"),
			unknownAttribute(payload, "documentation"),
			attributeViolation("source is no anyURI", invalidAttribute, `invalid "source"`).at(payload).setting(`source="%zz"`),
			attributeViolation("xml:lang is no language", invalidAttribute, "invalid xml:lang on xs:documentation").at(payload).setting(`xml:lang="1 2"`),
		},
	}
}

// standaloneFixture holds the violations that no edit of a valid fixture
// expresses: a child that a valid parent does not have, and compile-tier
// violations that span several components.
func standaloneFixture() fixture {
	const key = "violation"
	inComplexType := func(children func(n string) []*node) func(n string) []*node {
		return func(n string) []*node {
			return []*node{xs("complexType", `name="t`+n+`"`).with(children(n)...)}
		}
	}
	return fixture{name: "standalone", violations: []violation{
		childViolation("element in a complex type", contentModel, `invalid complexType child "element"`).at(key).
			alone(inComplexType(func(string) []*node { return []*node{xs("element", `name="e"`).as(key)} })),
		childViolation("annotation after another child", contentModel, `xs:"complexType" can contain xs:annotation only as its first child`).at(key).
			alone(inComplexType(func(string) []*node { return []*node{xs("sequence"), xs("annotation").as(key)} })),
		childViolation("child after content", contentModel, `invalid complexType child "attribute"`).at(key).
			alone(inComplexType(func(string) []*node {
				return []*node{xs("simpleContent").with(xs("extension", `base="xs:string"`)), xs("attribute", `name="a"`).as(key)}
			})),
		childViolation("child of a notation", contentModel, `xs:"notation" can contain only xs:annotation, not xs:"element"`).at(key).
			alone(func(n string) []*node {
				return []*node{xs("notation", `name="n`+n+`"`, `public="p"`).with(xs("element", `name="e"`).as(key))}
			}),
		childViolation("child of a facet", contentModel, `xs:"maxLength" can contain only xs:annotation, not xs:"simpleType"`).at(key).
			alone(func(n string) []*node {
				return []*node{xs("simpleType", `name="s`+n+`"`).with(xs("restriction", `base="xs:string"`).with(
					xs("maxLength", `value="5"`).with(xs("simpleType").as(key))))}
			}),
		childViolation("type in an element reference", contentModel, `xs:"element" can contain only xs:annotation, not xs:"complexType"`).at(key).
			alone(func(n string) []*node {
				return []*node{
					xs("element", `name="e`+n+`"`),
					xs("complexType", `name="t`+n+`"`).with(xs("sequence").with(xs("element", `ref="e`+n+`"`).with(xs("complexType").as(key)))),
				}
			}),
		childViolation("simpleType in a simpleContent extension", contentModel, "simpleContent extension cannot contain simpleType").at(key).
			alone(inComplexType(func(string) []*node {
				return []*node{xs("simpleContent").with(xs("extension", `base="xs:string"`).with(xs("simpleType").as(key)))}
			})),
		childViolation("import after a declaration", contentModel, `xs:"import" must precede global declarations`).at(key).
			alone(func(n string) []*node {
				return []*node{xs("element", `name="e`+n+`"`), xs("import", `namespace="urn:late-`+n+`"`).as(key)}
			}),
		childViolation("xs:all element in xs:all", contentModel, "xs:all can contain only element particles").at(key).
			alone(inComplexType(func(string) []*node { return []*node{xs("all").with(xs("any").as(key))} })),
		attributeViolation("id used twice", invalidAttribute, "duplicate schema id").at(key).
			alone(func(n string) []*node {
				return []*node{xs("element", `name="e`+n+`"`, `id="i`+n+`"`), xs("attribute", `name="a`+n+`"`, `id="i`+n+`"`).as(key)}
			}),
		attributeViolation("source of appinfo is no anyURI", invalidAttribute, `invalid "source"`).at(key).
			alone(func(string) []*node { return []*node{xs("annotation").with(xs("appinfo", `source="%zz"`).as(key))} }),
		parseViolation(stageText, "text in an annotation-only element", contentModel, `xs:"anyAttribute" cannot contain text`).at(key).
			alone(inComplexType(func(string) []*node {
				wildcard := xs("anyAttribute").as(key)
				wildcard.tail = characterData
				return []*node{wildcard}
			})),

		compileViolation("duplicate top-level element", duplicate, "duplicate").at(key).
			alone(func(n string) []*node {
				return []*node{xs("element", `name="e`+n+`"`), xs("element", `name="e`+n+`"`).as(key)}
			}),
		compileViolation("duplicate attribute use", duplicate, "duplicate attribute use").at(key).
			alone(inComplexType(func(string) []*node {
				return []*node{xs("attribute", `name="a"`), xs("attribute", `name="a"`).as(key)}
			})),
		compileViolation("simple type restricting itself", reference, "cyclic").at(key).
			alone(func(n string) []*node {
				return []*node{xs("simpleType", `name="s`+n+`"`).as(key).with(xs("restriction", `base="s`+n+`"`))}
			}),
		compileViolation("restriction of a final simple type", reference, "base simple type final blocks restriction").at(key).
			alone(func(n string) []*node {
				return []*node{
					xs("simpleType", `name="f`+n+`"`, `final="restriction"`).with(xs("restriction", `base="xs:string"`)),
					xs("simpleType", `name="s`+n+`"`).with(xs("restriction", `base="f`+n+`"`).as(key)),
				}
			}),
		compileViolation("ambiguous content model", contentModel, "UPA violation").at(key).unlocated().
			alone(inComplexType(func(string) []*node {
				return []*node{xs("sequence").as(key).with(
					xs("element", `name="e"`, `minOccurs="0"`),
					xs("element", `name="e"`))}
			})),
		compileViolation("group reference resolving to xs:all in a sequence", contentModel, "xs:all").at(key).
			alone(func(n string) []*node {
				return []*node{
					xs("group", `name="g`+n+`"`).with(xs("all").with(xs("element", `name="e"`))),
					xs("complexType", `name="t`+n+`"`).with(xs("sequence").with(xs("group", `ref="g`+n+`"`).as(key))),
				}
			}),
		compileViolation("enumeration value outside the base type", facetCode, "invalid facet value").at(key).
			alone(func(n string) []*node {
				return []*node{xs("simpleType", `name="s`+n+`"`).with(xs("restriction", `base="xs:int"`).with(
					xs("enumeration", `value="abc"`).as(key)))}
			}),
		compileViolation("occurrence of an xs:all particle above one", occurrence, "xs:all").at(key).
			alone(inComplexType(func(string) []*node {
				return []*node{xs("all").with(xs("element", `name="e"`, `maxOccurs="2"`).as(key))}
			})),
	}}
}
