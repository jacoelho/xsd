package loadmerge

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// Kind enumerates kind values.
type Kind int

const (
	MergeInclude Kind = iota
	MergeImport
)

// NamespaceRemapMode enumerates namespace remap mode values.
type NamespaceRemapMode int

const (
	RemapNamespace NamespaceRemapMode = iota
	KeepNamespace
)

type globalDeclKey struct {
	name model.QName
	kind parser.GlobalDeclKind
}

type mergeContext struct {
	target              *parser.Schema
	source              *parser.Schema
	remapQName          func(model.QName) model.QName
	opts                model.CopyOptions
	isImport            bool
	needsNamespaceRemap bool
}

// SchemaMerger applies include/import merges to parsed schemas.
type SchemaMerger interface {
	Merge(target, source *parser.Schema, kind Kind, remap NamespaceRemapMode, insertAt int) error
}

// DefaultMerger applies include/import merge semantics over parsed schemas.
type DefaultMerger struct{}

func newMergeContext(target, source *parser.Schema, kind Kind, remap NamespaceRemapMode) mergeContext {
	isImport := kind == MergeImport
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
		target:              target,
		source:              source,
		isImport:            isImport,
		needsNamespaceRemap: needsNamespaceRemap,
		remapQName:          remapQName,
		opts:                opts,
	}
}
