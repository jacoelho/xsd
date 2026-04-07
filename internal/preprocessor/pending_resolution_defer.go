package preprocessor

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
