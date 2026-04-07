package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

func (l *Loader) resolvePendingImportsFor(sourceKey loadKey) error {
	return ResolvePending(sourceKey, PendingResolveCallbacks[loadKey, *parser.Schema, map[loadKey]*stagedPendingTarget]{
		Inputs: func(sourceKey loadKey) (*Tracking[loadKey], []Directive[loadKey], *parser.Schema, error) {
			sourceEntry, pendingDirectives, source, err := l.pendingResolutionInputs(sourceKey)
			if sourceEntry == nil {
				return nil, nil, nil, err
			}
			return &sourceEntry.pending, pendingDirectives, source, err
		},
		Stage: l.stagePendingTargets,
		Apply: func(directives []Directive[loadKey], source *parser.Schema, staged map[loadKey]*stagedPendingTarget) error {
			return ApplyPending(directives, source, staged, PendingApplyCallbacks[loadKey, *parser.Schema, map[loadKey]*stagedPendingTarget, *stagedPendingTarget]{
				Target: func(staged map[loadKey]*stagedPendingTarget, key loadKey) (*stagedPendingTarget, error) {
					target := staged[key]
					if target == nil {
						return nil, fmt.Errorf("pending directive target not staged: %s", key.systemID)
					}
					return target, nil
				},
				Include: func(directive Directive[loadKey], source *parser.Schema, target *stagedPendingTarget) error {
					return l.applyPendingInclude(directive, source, target)
				},
				Import: func(directive Directive[loadKey], source *parser.Schema, target *stagedPendingTarget) error {
					return l.applyPendingImport(directive, source, target)
				},
			})
		},
		Commit: func(staged map[loadKey]*stagedPendingTarget) error {
			return l.commitStagedTargets(staged)
		},
		MarkMerged:     l.markPendingMerged,
		ResolveTargets: l.resolvePendingTargets,
	})
}
