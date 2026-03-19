package merge

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// Kind identifies how a parsed schema should be combined with another.
type Kind int

const (
	Include Kind = iota
	Import
)

// NamespaceMode controls chameleon-include remapping.
type NamespaceMode int

const (
	RemapNamespace NamespaceMode = iota
	KeepNamespace
)

type globalDeclKey struct {
	name model.QName
	kind parser.GlobalDeclKind
}

type mergeContext struct {
	targetGraph         *parser.SchemaGraph
	targetMeta          *parser.SchemaMeta
	sourceGraph         *parser.SchemaGraph
	sourceMeta          *parser.SchemaMeta
	remapQName          func(model.QName) model.QName
	opts                model.CopyOptions
	isImport            bool
	needsNamespaceRemap bool
}

func newMergeContext(target, source *parser.Schema, kind Kind, remap NamespaceMode) mergeContext {
	isImport := kind == Import
	needsNamespaceRemap := remap == RemapNamespace
	remapQName := func(qname model.QName) model.QName {
		if needsNamespaceRemap && qname.Namespace == "" {
			return model.QName{
				Namespace: target.TargetNamespace,
				Local:     qname.Local,
			}
		}
		return qname
	}

	sourceNamespace := source.TargetNamespace
	if !isImport && needsNamespaceRemap {
		sourceNamespace = target.TargetNamespace
	}

	opts := model.CopyOptions{
		SourceNamespace: sourceNamespace,
		RemapQName:      remapQName,
	}
	opts = model.WithGraphMemo(opts)

	return mergeContext{
		targetGraph:         &target.SchemaGraph,
		targetMeta:          &target.SchemaMeta,
		sourceGraph:         &source.SchemaGraph,
		sourceMeta:          &source.SchemaMeta,
		isImport:            isImport,
		needsNamespaceRemap: needsNamespaceRemap,
		remapQName:          remapQName,
		opts:                opts,
	}
}
