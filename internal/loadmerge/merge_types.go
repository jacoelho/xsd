package loadmerge

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type Kind int

const (
	MergeInclude Kind = iota
	MergeImport
)

type NamespaceRemapMode int

const (
	RemapNamespace NamespaceRemapMode = iota
	KeepNamespace
)

type globalDeclKey struct {
	name types.QName
	kind parser.GlobalDeclKind
}

type mergeContext struct {
	target              *parser.Schema
	source              *parser.Schema
	remapQName          func(types.QName) types.QName
	opts                types.CopyOptions
	isImport            bool
	needsNamespaceRemap bool
}

// SchemaMerger applies include/import merges to parsed schemas.
type SchemaMerger interface {
	Merge(target, source *parser.Schema, kind Kind, remap NamespaceRemapMode, insertAt int) error
}

type DefaultMerger struct{}

func newMergeContext(target, source *parser.Schema, kind Kind, remap NamespaceRemapMode) mergeContext {
	isImport := kind == MergeImport
	needsNamespaceRemap := remap == RemapNamespace
	remapQName := func(qname types.QName) types.QName {
		if needsNamespaceRemap && qname.Namespace.IsEmpty() {
			return types.QName{
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

	opts := types.CopyOptions{
		SourceNamespace: sourceNamespace,
		RemapQName:      remapQName,
	}
	opts = types.WithGraphMemo(opts)

	return mergeContext{
		target:              target,
		source:              source,
		isImport:            isImport,
		needsNamespaceRemap: needsNamespaceRemap,
		remapQName:          remapQName,
		opts:                opts,
	}
}
