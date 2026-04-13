package compiler

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
)

type stagedPendingTarget struct {
	schema          *parser.Schema
	includeInserted []int
}

// PendingResolveCallbacks supplies the caller-owned state and side effects needed to
// resolve one source's deferred directives.
type PendingResolveCallbacks[K comparable, S any, T any] struct {
	Inputs         func(K) (*Tracking[K], []Directive[K], S, error)
	Stage          func([]Directive[K]) (T, error)
	Apply          func([]Directive[K], S, T) error
	Commit         func(T) error
	MarkMerged     func(K, []Directive[K])
	ResolveTargets func([]Directive[K]) error
}

// ResolvePending handles the generic staged-apply-commit flow for one source's
// deferred directives.
func ResolvePending[K comparable, S any, T any](sourceKey K, callbacks PendingResolveCallbacks[K, S, T]) error {
	tracking, directives, source, err := callbacks.Inputs(sourceKey)
	if err != nil || len(directives) == 0 {
		return err
	}

	staged, err := callbacks.Stage(directives)
	if err != nil {
		return err
	}
	if err := callbacks.Apply(directives, source, staged); err != nil {
		return err
	}
	if err := callbacks.Commit(staged); err != nil {
		return err
	}
	if callbacks.MarkMerged != nil {
		callbacks.MarkMerged(sourceKey, directives)
	}
	if tracking != nil {
		tracking.Clear()
	}
	if callbacks.ResolveTargets != nil {
		return callbacks.ResolveTargets(directives)
	}
	return nil
}

// PendingTargetCallbacks supplies the caller-owned tracking and recursive resolution
// behavior for one set of deferred directive targets.
type PendingTargetCallbacks[K comparable] struct {
	Tracking func(K) (*Tracking[K], error)
	Resolve  func(K) error
	Label    func(K) string
}

// PendingApplyCallbacks supplies caller-owned staged-target lookup and per-kind merge behavior.
type PendingApplyCallbacks[K comparable, S any, T any, U any] struct {
	Target  func(T, K) (U, error)
	Include func(Directive[K], S, U) error
	Import  func(Directive[K], S, U) error
}

// ApplyPending looks up each staged target and dispatches deferred directives by kind.
func ApplyPending[K comparable, S any, T any, U any](directives []Directive[K], source S, staged T, callbacks PendingApplyCallbacks[K, S, T, U]) error {
	for _, directive := range directives {
		target, err := callbacks.Target(staged, directive.TargetKey)
		if err != nil {
			return err
		}
		switch directive.Kind {
		case 0:
			if callbacks.Include == nil {
				return fmt.Errorf("missing include callback")
			}
			if err := callbacks.Include(directive, source, target); err != nil {
				return err
			}
		case 1:
			if callbacks.Import == nil {
				return fmt.Errorf("missing import callback")
			}
			if err := callbacks.Import(directive, source, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown pending directive kind: %d", directive.Kind)
		}
	}
	return nil
}

// ResolvePendingTargets decrements unresolved target counts and triggers recursive
// resolution when a target becomes ready.
func ResolvePendingTargets[K comparable](directives []Directive[K], callbacks PendingTargetCallbacks[K]) error {
	for _, directive := range directives {
		tracking, err := callbacks.Tracking(directive.TargetKey)
		if err != nil {
			return err
		}
		label := ""
		if callbacks.Label != nil {
			label = callbacks.Label(directive.TargetKey)
		}
		if err := tracking.Decrement(label); err != nil {
			return err
		}
		if tracking.Count == 0 && callbacks.Resolve != nil {
			if err := callbacks.Resolve(directive.TargetKey); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *Loader) applyPendingInclude(directive Directive[loadKey], source *parser.Schema, target *stagedPendingTarget) error {
	includingNS := directive.TargetKey.etn
	includeInfo := parser.IncludeInfo{
		SchemaLocation: directive.SchemaLocation,
		DeclIndex:      directive.IncludeDeclIndex,
		IncludeIndex:   directive.IncludeIndex,
	}
	plan, err := PlanInclude(includingNS, target.includeInserted, target.schema, includeInfo, directive.SchemaLocation, source)
	if err != nil {
		return err
	}
	inserted, err := ApplyPlanned(target.schema, source, plan, "included", directive.SchemaLocation)
	if err != nil {
		return err
	}
	return RecordIncludeInserted(target.includeInserted, directive.IncludeIndex, inserted)
}

func (l *Loader) applyPendingImport(directive Directive[loadKey], source *parser.Schema, target *stagedPendingTarget) error {
	plan, err := PlanImport(directive.SchemaLocation, directive.ExpectedNamespace, source, len(target.schema.GlobalDecls))
	if err != nil {
		return err
	}
	if _, err := ApplyPlanned(target.schema, source, plan, "imported", directive.SchemaLocation); err != nil {
		return err
	}
	return nil
}

func (l *Loader) commitStagedTargets(staged map[loadKey]*stagedPendingTarget) error {
	for key, stagedTarget := range staged {
		target, err := l.schemaForKeyStrict(key)
		if err != nil {
			return err
		}
		*target = *stagedTarget.schema
		if entry, ok := l.state.entry(key); ok && entry != nil {
			entry.includeInserted = stagedTarget.includeInserted
		}
	}
	return nil
}

func (l *Loader) markPendingMerged(sourceKey loadKey, pendingDirectives []Directive[loadKey]) {
	for _, directive := range pendingDirectives {
		l.imports.MarkMerged(directive.Kind, directive.TargetKey, sourceKey)
	}
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

func (l *Loader) pendingResolutionInputs(sourceKey loadKey) (*schemaEntry, []Directive[loadKey], *parser.Schema, error) {
	sourceEntry := l.state.ensureEntry(sourceKey)
	if sourceEntry.pending.Count > 0 {
		return sourceEntry, nil, nil, nil
	}
	pendingDirectives := sourceEntry.pending.Directives
	if len(pendingDirectives) == 0 {
		return sourceEntry, nil, nil, nil
	}
	source := l.state.schemaForKey(sourceKey)
	if source == nil {
		return nil, nil, nil, fmt.Errorf("pending import source not found: %s", sourceKey.systemID)
	}
	return sourceEntry, pendingDirectives, source, nil
}

func (l *Loader) stagePendingTargets(pendingDirectives []Directive[loadKey]) (map[loadKey]*stagedPendingTarget, error) {
	staged := make(map[loadKey]*stagedPendingTarget, len(pendingDirectives))
	for _, directive := range pendingDirectives {
		if _, ok := staged[directive.TargetKey]; ok {
			continue
		}
		target, err := l.schemaForKeyStrict(directive.TargetKey)
		if err != nil {
			return nil, err
		}
		entry, ok := l.state.entry(directive.TargetKey)
		if !ok || entry == nil {
			return nil, fmt.Errorf("pending directive tracking missing for %s", directive.TargetKey.systemID)
		}
		staged[directive.TargetKey] = &stagedPendingTarget{
			schema:          parser.CloneSchemaForMerge(target),
			includeInserted: slices.Clone(entry.includeInserted),
		}
	}
	return staged, nil
}

func (l *Loader) schemaForKeyStrict(key loadKey) (*parser.Schema, error) {
	target := l.state.schemaForKey(key)
	if target == nil {
		return nil, fmt.Errorf("pending directive target not found: %s", key.systemID)
	}
	return target, nil
}
