package source

func (l *SchemaLoader) resolvePendingTargets(pendingDirectives []pendingDirective) error {
	for _, directive := range pendingDirectives {
		if err := l.decrementPendingAndResolve(directive.targetKey); err != nil {
			return err
		}
	}
	return nil
}

func (l *SchemaLoader) decrementPendingAndResolve(targetKey loadKey) error {
	targetEntry := l.state.ensureEntry(targetKey)
	if err := decPendingCount(targetEntry, targetKey); err != nil {
		return err
	}
	if targetEntry.pendingCount == 0 {
		if err := l.resolvePendingImportsFor(targetKey); err != nil {
			return err
		}
	}
	return nil
}
