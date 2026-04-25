package schemaast

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/xmlstream"
)

// LoadDocumentSetFS loads a root schema and referenced import/include documents into parse-only documents.
func LoadDocumentSetFS(fsys fs.FS, location string, opts ...xmlstream.Option) (*DocumentSet, error) {
	if fsys == nil {
		return nil, fmt.Errorf("schema document set: nil fs")
	}
	if strings.TrimSpace(location) == "" {
		return nil, fmt.Errorf("schema document set: empty location")
	}
	loader := documentSetLoader{
		fsys:    fsys,
		options: opts,
		seen:    make(map[string]bool),
	}
	if err := loader.load(location, NamespaceEmpty); err != nil {
		return nil, err
	}
	return &DocumentSet{Documents: loader.documents}, nil
}

type documentSetLoader struct {
	fsys      fs.FS
	options   []xmlstream.Option
	seen      map[string]bool
	documents []SchemaDocument
}

func (l *documentSetLoader) load(location string, includingNamespace NamespaceURI) error {
	location = path.Clean(location)
	if l.seen[location] {
		return nil
	}
	l.seen[location] = true

	file, err := l.fsys.Open(location)
	if err != nil {
		return fmt.Errorf("open schema %s: %w", location, err)
	}
	defer file.Close()

	result, err := ParseDocumentWithImportsOptions(file, l.options...)
	if err != nil {
		return fmt.Errorf("parse schema %s: %w", location, err)
	}
	doc := *result.Document
	doc.Location = location
	if doc.TargetNamespace == NamespaceEmpty && includingNamespace != NamespaceEmpty {
		remapDocumentNamespace(&doc, includingNamespace)
	}
	l.documents = append(l.documents, doc)

	for _, directive := range doc.Directives {
		switch directive.Kind {
		case DirectiveInclude:
			target := strings.TrimSpace(directive.Include.SchemaLocation)
			if target == "" {
				return fmt.Errorf("include missing schemaLocation")
			}
			if err := l.load(resolveDocumentLocation(location, target), doc.TargetNamespace); err != nil {
				return err
			}
		case DirectiveImport:
			target := strings.TrimSpace(directive.Import.SchemaLocation)
			if target == "" {
				continue
			}
			if err := l.load(resolveDocumentLocation(location, target), NamespaceEmpty); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveDocumentLocation(base, loc string) string {
	if loc == "" || path.IsAbs(loc) {
		return path.Clean(loc)
	}
	return path.Clean(path.Join(path.Dir(base), loc))
}

func remapDocumentNamespace(doc *SchemaDocument, target NamespaceURI) {
	doc.TargetNamespace = target
	remapNamespaceContexts(doc.NamespaceContexts, target)
	for i := range doc.Decls {
		remapTopLevelDecl(&doc.Decls[i], target, doc.Defaults)
	}
}

// RemapChameleonDocument applies an including schema target namespace to a
// no-namespace include document. It rewrites only lexical declaration data.
func RemapChameleonDocument(doc *SchemaDocument, target NamespaceURI) {
	if doc == nil || target == NamespaceEmpty {
		return
	}
	remapDocumentNamespace(doc, target)
}

func remapNamespaceContexts(contexts []NamespaceContext, target NamespaceURI) {
	for i := range contexts {
		bindings := contexts[i].Bindings
		foundDefault := false
		for j := range bindings {
			if bindings[j].Prefix != "" {
				continue
			}
			foundDefault = true
			if bindings[j].URI == NamespaceEmpty {
				bindings[j].URI = target
			}
		}
		if !foundDefault {
			bindings = append(bindings, NamespaceBinding{URI: target})
		}
		slices.SortFunc(bindings, func(a, b NamespaceBinding) int {
			return strings.Compare(a.Prefix, b.Prefix)
		})
		contexts[i].Bindings = bindings
	}
}

func remapTopLevelDecl(decl *TopLevelDecl, target NamespaceURI, defaults SchemaDefaults) {
	decl.Name = remapEmptyQName(decl.Name, target)
	switch {
	case decl.SimpleType != nil:
		remapSimpleType(decl.SimpleType, target)
	case decl.ComplexType != nil:
		remapComplexType(decl.ComplexType, target, defaults)
	case decl.Element != nil:
		remapElementDecl(decl.Element, target, defaults)
	case decl.Attribute != nil:
		remapAttributeDecl(decl.Attribute, target, defaults)
	case decl.Group != nil:
		remapGroupDecl(decl.Group, target, defaults)
	case decl.AttributeGroup != nil:
		remapAttributeGroupDecl(decl.AttributeGroup, target, defaults)
	case decl.Notation != nil:
		decl.Notation.Name = remapEmptyQName(decl.Notation.Name, target)
		decl.Notation.SourceNamespace = target
	}
}

func remapSimpleType(decl *SimpleTypeDecl, target NamespaceURI) {
	decl.Name = remapEmptyQName(decl.Name, target)
	decl.Base = remapEmptyQName(decl.Base, target)
	decl.ItemType = remapEmptyQName(decl.ItemType, target)
	decl.SourceNamespace = target
	for i := range decl.MemberTypes {
		decl.MemberTypes[i] = remapEmptyQName(decl.MemberTypes[i], target)
	}
	if decl.InlineBase != nil {
		remapSimpleType(decl.InlineBase, target)
	}
	if decl.InlineItem != nil {
		remapSimpleType(decl.InlineItem, target)
	}
	for i := range decl.InlineMembers {
		remapSimpleType(&decl.InlineMembers[i], target)
	}
}

func remapComplexType(decl *ComplexTypeDecl, target NamespaceURI, defaults SchemaDefaults) {
	decl.Name = remapEmptyQName(decl.Name, target)
	decl.Base = remapEmptyQName(decl.Base, target)
	decl.SourceNamespace = target
	for i := range decl.Attributes {
		remapAttributeUseDecl(&decl.Attributes[i], target, defaults)
	}
	for i := range decl.AttributeGroups {
		decl.AttributeGroups[i] = remapEmptyQName(decl.AttributeGroups[i], target)
	}
	if decl.AnyAttribute != nil {
		decl.AnyAttribute.TargetNamespace = target
	}
	if decl.Particle != nil {
		remapParticleDecl(decl.Particle, target, defaults)
	}
	if decl.SimpleType != nil {
		remapSimpleType(decl.SimpleType, target)
	}
}

func remapElementDecl(decl *ElementDecl, target NamespaceURI, defaults SchemaDefaults) {
	if decl.Global || localElementNameQualified(decl.Form, defaults) {
		decl.Name = remapEmptyQName(decl.Name, target)
	}
	decl.Ref = remapEmptyQName(decl.Ref, target)
	decl.Type.Name = remapEmptyQName(decl.Type.Name, target)
	decl.SubstitutionGroup = remapEmptyQName(decl.SubstitutionGroup, target)
	decl.SourceNamespace = target
	if decl.Type.Simple != nil {
		remapSimpleType(decl.Type.Simple, target)
	}
	if decl.Type.Complex != nil {
		remapComplexType(decl.Type.Complex, target, defaults)
	}
	for i := range decl.Identity {
		decl.Identity[i].Name = remapEmptyQName(decl.Identity[i].Name, target)
		decl.Identity[i].Refer = remapEmptyQName(decl.Identity[i].Refer, target)
	}
}

func remapAttributeDecl(decl *AttributeDecl, target NamespaceURI, defaults SchemaDefaults) {
	if decl.Global || localAttributeNameQualified(decl.Form, defaults) {
		decl.Name = remapEmptyQName(decl.Name, target)
	}
	decl.Ref = remapEmptyQName(decl.Ref, target)
	decl.Type.Name = remapEmptyQName(decl.Type.Name, target)
	decl.SourceNamespace = target
	if decl.Type.Simple != nil {
		remapSimpleType(decl.Type.Simple, target)
	}
}

func remapAttributeUseDecl(use *AttributeUseDecl, target NamespaceURI, defaults SchemaDefaults) {
	if use.Attribute != nil {
		remapAttributeDecl(use.Attribute, target, defaults)
	}
	use.AttributeGroup = remapEmptyQName(use.AttributeGroup, target)
}

func remapGroupDecl(decl *GroupDecl, target NamespaceURI, defaults SchemaDefaults) {
	decl.Name = remapEmptyQName(decl.Name, target)
	decl.Ref = remapEmptyQName(decl.Ref, target)
	decl.SourceNamespace = target
	if decl.Particle != nil {
		remapParticleDecl(decl.Particle, target, defaults)
	}
}

func remapAttributeGroupDecl(decl *AttributeGroupDecl, target NamespaceURI, defaults SchemaDefaults) {
	decl.Name = remapEmptyQName(decl.Name, target)
	decl.Ref = remapEmptyQName(decl.Ref, target)
	decl.SourceNamespace = target
	for i := range decl.Attributes {
		remapAttributeUseDecl(&decl.Attributes[i], target, defaults)
	}
	for i := range decl.AttributeGroups {
		decl.AttributeGroups[i] = remapEmptyQName(decl.AttributeGroups[i], target)
	}
	if decl.AnyAttribute != nil {
		decl.AnyAttribute.TargetNamespace = target
	}
}

func remapParticleDecl(decl *ParticleDecl, target NamespaceURI, defaults SchemaDefaults) {
	if decl.Element != nil {
		remapElementDecl(decl.Element, target, defaults)
	}
	if decl.Wildcard != nil {
		decl.Wildcard.TargetNamespace = target
	}
	decl.GroupRef = remapEmptyQName(decl.GroupRef, target)
	for i := range decl.Children {
		remapParticleDecl(&decl.Children[i], target, defaults)
	}
}

func localElementNameQualified(form FormChoice, defaults SchemaDefaults) bool {
	switch form {
	case FormQualified:
		return true
	case FormUnqualified:
		return false
	default:
		return defaults.ElementFormDefault == Qualified
	}
}

func localAttributeNameQualified(form FormChoice, defaults SchemaDefaults) bool {
	switch form {
	case FormQualified:
		return true
	case FormUnqualified:
		return false
	default:
		return defaults.AttributeFormDefault == Qualified
	}
}

func remapEmptyQName(name QName, target NamespaceURI) QName {
	if name.IsZero() || name.Namespace != NamespaceEmpty {
		return name
	}
	name.Namespace = target
	return name
}
