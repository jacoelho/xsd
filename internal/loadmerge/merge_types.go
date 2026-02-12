package loadmerge

import (
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
)

// Kind defines an exported type.
type Kind int

const (
	// MergeInclude is an exported constant.
	MergeInclude Kind = iota
	// MergeImport is an exported constant.
	MergeImport
)

// NamespaceRemapMode defines an exported type.
type NamespaceRemapMode int

const (
	// RemapNamespace is an exported constant.
	RemapNamespace NamespaceRemapMode = iota
	// KeepNamespace is an exported constant.
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

// DefaultMerger defines an exported type.
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
