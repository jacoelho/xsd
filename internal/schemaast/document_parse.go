package schemaast

import (
	"fmt"
	"io"
	"slices"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

// ParseDocumentWithImportsOptions parses an XSD document into parse-only declarations.
func ParseDocumentWithImportsOptions(r io.Reader, opts ...xmlstream.Option) (*ParseResult, error) {
	return ParseDocumentWithImportsOptionsWithPool(r, NewDocumentPool(), opts...)
}

// ParseDocumentWithImportsOptionsWithPool parses an XSD document into parse-only declarations with an explicit document pool.
func ParseDocumentWithImportsOptionsWithPool(r io.Reader, pool *DocumentPool, opts ...xmlstream.Option) (*ParseResult, error) {
	if pool == nil {
		pool = NewDocumentPool()
	}
	buffered := acquireParseReader(r)
	defer releaseParseReader(buffered)

	doc := pool.Acquire()
	defer pool.Release(doc)
	if err := parseIntoWithOptions(buffered, doc, opts...); err != nil {
		return nil, newParseError(err)
	}
	parser := newDocumentParser(doc)
	document, err := parser.parse()
	if err != nil {
		return nil, wrapParseErr(err)
	}
	return &ParseResult{
		Document:   document,
		Directives: slices.Clone(document.Directives),
		Imports:    slices.Clone(document.Imports),
		Includes:   slices.Clone(document.Includes),
	}, nil
}

type documentParser struct {
	doc        *Document
	result     *SchemaDocument
	contextIDs map[string]NamespaceContextID
	ids        map[string]string
}

func newDocumentParser(doc *Document) *documentParser {
	return &documentParser{
		doc:        doc,
		contextIDs: make(map[string]NamespaceContextID),
		ids:        make(map[string]string),
	}
}

func (p *documentParser) parse() (*SchemaDocument, error) {
	root := p.doc.DocumentElement()
	if root == InvalidNode {
		return nil, fmt.Errorf("empty document")
	}
	if p.doc.NamespaceURI(root) != value.XSDNamespace || p.doc.LocalName(root) != "schema" {
		return nil, fmt.Errorf("root element must be xs:schema, got {%s}%s", p.doc.NamespaceURI(root), p.doc.LocalName(root))
	}
	if err := validateSchemaAttributeNamespaces(p.doc, root); err != nil {
		return nil, err
	}
	target, defaults, err := p.parseSchemaAttrs(root)
	if err != nil {
		return nil, err
	}
	if err := p.validateSchemaIDs(root); err != nil {
		return nil, err
	}
	p.result = &SchemaDocument{
		TargetNamespace: target,
		Defaults:        defaults,
	}
	_ = p.contextID(root)

	for _, child := range p.xsdChildren(root) {
		switch p.doc.LocalName(child) {
		case "annotation":
			continue
		case "import":
			if err := p.parseImport(child); err != nil {
				return nil, err
			}
		case "include":
			if err := p.parseInclude(child); err != nil {
				return nil, err
			}
		case "redefine":
			return nil, fmt.Errorf("redefine is not supported")
		case "simpleType", "complexType", "element", "attribute", "group", "attributeGroup", "notation":
			decl, err := p.parseTopLevelDecl(child)
			if err != nil {
				return nil, err
			}
			p.result.Decls = append(p.result.Decls, decl)
		default:
			return nil, fmt.Errorf("unexpected top-level element '%s'", p.doc.LocalName(child))
		}
	}
	return p.result, nil
}

func (p *documentParser) validateSchemaIDs(elem NodeID) error {
	if p.doc.NamespaceURI(elem) == value.XSDNamespace {
		if id := p.attr(elem, "id"); id != "" || p.hasAttr(elem, "id") {
			name := p.doc.LocalName(elem)
			if err := value.ValidateXSDNCName(id); err != nil {
				return fmt.Errorf("%s element has invalid id attribute '%s': must be a valid NCName", name, id)
			}
			if existing, ok := p.ids[id]; ok {
				return fmt.Errorf("%s element has duplicate id attribute '%s' (already used by %s)", name, id, existing)
			}
			p.ids[id] = name
		}
	}
	for _, child := range p.doc.Children(elem) {
		if err := p.validateSchemaIDs(child); err != nil {
			return err
		}
	}
	return nil
}

func (p *documentParser) parseSchemaAttrs(root NodeID) (NamespaceURI, SchemaDefaults, error) {
	var defaults SchemaDefaults
	target := NamespaceURI(p.attr(root, "targetNamespace"))
	if p.hasAttr(root, "targetNamespace") && target == "" {
		return "", defaults, fmt.Errorf("targetNamespace attribute cannot be empty (must be absent or have a non-empty value)")
	}
	if p.hasAttr(root, "elementFormDefault") && TrimXMLWhitespace(p.rawAttr(root, "elementFormDefault")) == "" {
		return "", defaults, fmt.Errorf("elementFormDefault attribute cannot be empty")
	}
	if elemForm := p.attr(root, "elementFormDefault"); elemForm != "" {
		form, err := parseSchemaForm(elemForm)
		if err != nil {
			return "", defaults, fmt.Errorf("invalid elementFormDefault attribute value '%s': %w", elemForm, err)
		}
		defaults.ElementFormDefault = form
	}
	if p.hasAttr(root, "attributeFormDefault") && TrimXMLWhitespace(p.rawAttr(root, "attributeFormDefault")) == "" {
		return "", defaults, fmt.Errorf("attributeFormDefault attribute cannot be empty")
	}
	if attrForm := p.attr(root, "attributeFormDefault"); attrForm != "" {
		form, err := parseSchemaForm(attrForm)
		if err != nil {
			return "", defaults, fmt.Errorf("invalid attributeFormDefault attribute value '%s': %w", attrForm, err)
		}
		defaults.AttributeFormDefault = form
	}
	if block := p.attr(root, "blockDefault"); block != "" {
		set, err := parseDerivationSetWithValidation(
			block,
			DerivationSet(DerivationSubstitution|DerivationExtension|DerivationRestriction),
		)
		if err != nil {
			return "", defaults, fmt.Errorf("invalid blockDefault attribute value '%s': %w", block, err)
		}
		defaults.BlockDefault = set
	}
	if final := p.attr(root, "finalDefault"); final != "" {
		set, err := parseDerivationSetWithValidation(
			final,
			DerivationSet(DerivationExtension|DerivationRestriction|DerivationList|DerivationUnion),
		)
		if err != nil {
			return "", defaults, fmt.Errorf("invalid finalDefault attribute value '%s': %w", final, err)
		}
		defaults.FinalDefault = set
	}
	return target, defaults, nil
}

func (p *documentParser) parseImport(elem NodeID) error {
	info := ImportInfo{
		Namespace:      p.attr(elem, "namespace"),
		SchemaLocation: p.attr(elem, "schemaLocation"),
	}
	p.result.Imports = append(p.result.Imports, info)
	p.result.Directives = append(p.result.Directives, Directive{
		Kind:   DirectiveImport,
		Import: info,
	})
	return nil
}

func (p *documentParser) parseInclude(elem NodeID) error {
	info := IncludeInfo{
		SchemaLocation: p.attr(elem, "schemaLocation"),
		DeclIndex:      len(p.result.Decls),
		IncludeIndex:   len(p.result.Includes),
	}
	p.result.Includes = append(p.result.Includes, info)
	p.result.Directives = append(p.result.Directives, Directive{
		Kind:    DirectiveInclude,
		Include: info,
	})
	return nil
}

func (p *documentParser) parseTopLevelDecl(elem NodeID) (TopLevelDecl, error) {
	switch p.doc.LocalName(elem) {
	case "simpleType":
		typ, err := p.parseSimpleType(elem, true)
		if err != nil {
			return TopLevelDecl{}, err
		}
		return TopLevelDecl{
			Kind:       DeclSimpleType,
			Name:       typ.Name,
			SimpleType: typ,
			Origin:     typ.Origin,
		}, nil
	case "complexType":
		typ, err := p.parseComplexType(elem, true)
		if err != nil {
			return TopLevelDecl{}, err
		}
		return TopLevelDecl{
			Kind:        DeclComplexType,
			Name:        typ.Name,
			ComplexType: typ,
			Origin:      typ.Origin,
		}, nil
	case "element":
		elemDecl, err := p.parseElement(elem, true)
		if err != nil {
			return TopLevelDecl{}, err
		}
		return TopLevelDecl{
			Kind:    DeclElement,
			Name:    elemDecl.Name,
			Element: elemDecl,
			Origin:  elemDecl.Origin,
		}, nil
	case "attribute":
		attr, err := p.parseAttribute(elem, true)
		if err != nil {
			return TopLevelDecl{}, err
		}
		return TopLevelDecl{
			Kind:      DeclAttribute,
			Name:      attr.Name,
			Attribute: attr,
			Origin:    attr.Origin,
		}, nil
	case "group":
		group, err := p.parseGroup(elem, true)
		if err != nil {
			return TopLevelDecl{}, err
		}
		return TopLevelDecl{
			Kind:   DeclGroup,
			Name:   group.Name,
			Group:  group,
			Origin: group.Origin,
		}, nil
	case "attributeGroup":
		group, err := p.parseAttributeGroup(elem, true)
		if err != nil {
			return TopLevelDecl{}, err
		}
		return TopLevelDecl{
			Kind:           DeclAttributeGroup,
			Name:           group.Name,
			AttributeGroup: group,
			Origin:         group.Origin,
		}, nil
	case "notation":
		notation, err := p.parseNotation(elem)
		if err != nil {
			return TopLevelDecl{}, err
		}
		return TopLevelDecl{
			Kind:     DeclNotation,
			Name:     notation.Name,
			Notation: notation,
			Origin:   notation.Origin,
		}, nil
	default:
		return TopLevelDecl{}, fmt.Errorf("unexpected top-level element '%s'", p.doc.LocalName(elem))
	}
}
