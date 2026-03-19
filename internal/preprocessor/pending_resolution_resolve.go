package preprocessor

func (l *Loader) resolvePendingTargets(pendingDirectives []Directive[loadKey]) error {
	return ResolvePendingTargets(pendingDirectives, PendingTargetCallbacks[loadKey]{
		Tracking: func(targetKey loadKey) (*Tracking[loadKey], error) {
			targetEntry := l.state.ensureEntry(targetKey)
			return &targetEntry.pending, nil
		},
		Resolve: l.resolvePendingImportsFor,
		Label: func(targetKey loadKey) string {
			return targetKey.systemID
		},
	})
}
