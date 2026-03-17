package preprocessor

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// MergeKind identifies how a parsed schema should be combined with another.
type MergeKind int

const (
	MergeInclude MergeKind = iota
	MergeImport
)

// NamespaceRemapMode controls chameleon include remapping.
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

func newMergeContext(target, source *parser.Schema, kind MergeKind, remap NamespaceRemapMode) mergeContext {
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
