package preprocessor

import (
	"github.com/jacoelho/xsd/internal/parser"
)

func (l *Loader) deferImport(sourceKey, targetKey loadKey, schemaLocation, expectedNamespace string, journal *Journal[loadKey]) bool {
	directive := Directive[loadKey]{
		Kind:              parser.DirectiveImport,
		TargetKey:         targetKey,
		SchemaLocation:    schemaLocation,
		ExpectedNamespace: expectedNamespace,
	}
	return l.deferDirective(sourceKey, directive, journal)
}

func (l *Loader) deferInclude(sourceKey, targetKey loadKey, include parser.IncludeInfo, journal *Journal[loadKey]) bool {
	directive := Directive[loadKey]{
		Kind:             parser.DirectiveInclude,
		TargetKey:        targetKey,
		SchemaLocation:   include.SchemaLocation,
		IncludeDeclIndex: include.DeclIndex,
		IncludeIndex:     include.IncludeIndex,
	}
	return l.deferDirective(sourceKey, directive, journal)
}

func (l *Loader) deferDirective(sourceKey loadKey, directive Directive[loadKey], journal *Journal[loadKey]) bool {
	sourceEntry := l.state.ensureEntry(sourceKey)
	if !sourceEntry.pending.Append(directive) {
		return false
	}
	if journal != nil {
		journal.RecordAppendPendingDirective(directive.Kind, sourceKey, directive.TargetKey)
	}

	targetEntry := l.state.ensureEntry(directive.TargetKey)
	targetEntry.pending.Increment()
	if journal != nil {
		journal.RecordIncPendingCount(directive.TargetKey)
	}
	return true
}
