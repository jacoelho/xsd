package schema

import (
	"errors"
	"slices"
)

// wildcardMode identifies the namespace set represented by a wildcard.
type wildcardMode uint8

const (
	// WildcardAny allows every namespace.
	WildcardAny wildcardMode = iota
	// WildcardOther allows every non-empty namespace except OtherThan.
	WildcardOther
	// WildcardLocal allows only the empty namespace.
	WildcardLocal
	// WildcardTargetNamespace allows the single namespace in Namespaces.
	WildcardTargetNamespace
	// WildcardList allows the sorted namespace IDs in Namespaces.
	WildcardList
)

// ProcessContents identifies how wildcard matches are validated.
type ProcessContents uint8

const (
	// processStrict requires a declaration for wildcard matches.
	processStrict ProcessContents = iota
	// ProcessLax validates wildcard matches when a declaration is available.
	ProcessLax
	// ProcessSkip accepts wildcard matches without declaration validation.
	ProcessSkip
)

// wildcardRuntime is the runtime representation of an element or attribute wildcard.
type wildcardRuntime struct {
	Namespaces []namespaceID
	OtherThan  namespaceID
	Mode       wildcardMode
	Process    ProcessContents
}

// wildcardByID resolves and clones a wildcard from a wildcard table.
func wildcardByID(wildcards []wildcardRuntime, id WildcardID) (wildcardRuntime, bool) {
	if !validWildcardID(id, len(wildcards)) {
		return wildcardRuntime{}, false
	}
	return cloneWildcard(wildcards[id]), true
}

// WildcardView is a read-only validation view over a frozen wildcard.
type WildcardView struct {
	otherThan  string
	namespaces []string
	mode       wildcardMode
	process    ProcessContents
	valid      bool
}

// newWildcardView returns a read-only validation view over wildcard.
func newWildcardView(names *nameTable, wildcard *wildcardRuntime) WildcardView {
	if wildcard == nil {
		return WildcardView{}
	}
	view := WildcardView{
		mode:    wildcard.Mode,
		process: wildcard.Process,
		valid:   true,
	}
	switch wildcard.Mode {
	case WildcardAny, WildcardLocal:
		return view
	case WildcardOther:
		return newOtherWildcardView(view, names, wildcard.OtherThan)
	case WildcardTargetNamespace, WildcardList:
		return newFiniteWildcardView(view, names, wildcard.Namespaces)
	default:
		view.valid = false
		return view
	}
}

func newOtherWildcardView(view WildcardView, names *nameTable, otherThan namespaceID) WildcardView {
	if names == nil || !names.ValidNamespaceID(otherThan) {
		view.valid = false
		return view
	}
	view.otherThan = names.Namespace(otherThan)
	return view
}

func newFiniteWildcardView(view WildcardView, names *nameTable, namespaces []namespaceID) WildcardView {
	if names == nil {
		view.valid = false
		return view
	}
	view.namespaces = make([]string, len(namespaces))
	for i, ns := range namespaces {
		if !names.ValidNamespaceID(ns) {
			view.valid = false
			return view
		}
		view.namespaces[i] = names.Namespace(ns)
	}
	return view
}

// newWildcardViews returns read-only validation views over wildcards.
func newWildcardViews(names *nameTable, wildcards []wildcardRuntime) []WildcardView {
	out := make([]WildcardView, len(wildcards))
	for i := range wildcards {
		out[i] = newWildcardView(names, &wildcards[i])
	}
	return out
}

// Process reports the wildcard processContents mode.
func (v WildcardView) Process() ProcessContents {
	if !v.valid {
		return processStrict
	}
	return v.process
}

// AllowsURI reports whether the wildcard admits uri.
func (v WildcardView) AllowsURI(uri string) bool {
	if !v.valid {
		return false
	}
	switch v.mode {
	case WildcardAny:
		return true
	case WildcardOther:
		return uri != "" && uri != v.otherThan
	case WildcardLocal:
		return uri == ""
	case WildcardTargetNamespace:
		return len(v.namespaces) != 0 && v.namespaces[0] == uri
	case WildcardList:
		return slices.Contains(v.namespaces, uri)
	default:
		return false
	}
}

// wildcardViewByID returns a validation wildcard view from the frozen wildcard
// read projection table.
func wildcardViewByID(views []WildcardView, id WildcardID) (WildcardView, bool) {
	if !validWildcardID(id, len(views)) {
		return WildcardView{}, false
	}
	return views[id], true
}

// validateWildcard validates frozen wildcard metadata.
func validateWildcard(names *nameTable, w wildcardRuntime) error {
	if names == nil {
		return errors.New("wildcard requires name table")
	}
	if !validProcessContents(w.Process) {
		return errors.New("wildcard has invalid process contents")
	}
	return validateWildcardNamespaceShape(names, w)
}

func validProcessContents(process ProcessContents) bool {
	switch process {
	case processStrict, ProcessLax, ProcessSkip:
		return true
	default:
		return false
	}
}

func validateWildcardNamespaceShape(names *nameTable, w wildcardRuntime) error {
	switch w.Mode {
	case WildcardAny, WildcardLocal:
		return validateInactiveWildcardNamespaces(w)
	case WildcardOther:
		return validateOtherWildcardNamespace(names, w)
	case WildcardTargetNamespace:
		return validateTargetWildcardNamespace(names, w)
	case WildcardList:
		return validateListWildcardNamespaces(names, w)
	default:
		return errors.New("wildcard has invalid mode")
	}
}

func validateInactiveWildcardNamespaces(w wildcardRuntime) error {
	if len(w.Namespaces) != 0 || w.OtherThan != emptyNamespaceID {
		return errors.New("wildcard stores inactive namespace fields")
	}
	return nil
}

func validateOtherWildcardNamespace(names *nameTable, w wildcardRuntime) error {
	if len(w.Namespaces) != 0 || !names.ValidNamespaceID(w.OtherThan) {
		return errors.New("wildcard other namespace is invalid")
	}
	return nil
}

func validateTargetWildcardNamespace(names *nameTable, w wildcardRuntime) error {
	if len(w.Namespaces) != 1 || w.OtherThan != emptyNamespaceID || !names.ValidNamespaceID(w.Namespaces[0]) {
		return errors.New("targetNamespace wildcard has invalid namespace")
	}
	return nil
}

func validateListWildcardNamespaces(names *nameTable, w wildcardRuntime) error {
	if w.OtherThan != emptyNamespaceID {
		return errors.New("namespace list wildcard stores inactive other namespace")
	}
	if !validWildcardNamespaceList(names, w.Namespaces) {
		return errors.New("namespace list wildcard is invalid")
	}
	return nil
}

func validWildcardNamespaceList(names *nameTable, namespaces []namespaceID) bool {
	var prev namespaceID
	for i, ns := range namespaces {
		if !names.ValidNamespaceID(ns) {
			return false
		}
		if i != 0 && ns <= prev {
			return false
		}
		prev = ns
	}
	return true
}

// normalizeNamespaceList sorts namespaces and removes duplicate IDs.
func normalizeNamespaceList(namespaces []namespaceID) []namespaceID {
	slices.Sort(namespaces)
	return slices.Compact(namespaces)
}

// wildcardNamespaceEqual reports whether two wildcards represent the same
// namespace set, ignoring process contents.
func wildcardNamespaceEqual(a, b wildcardRuntime) bool {
	if a.Mode != b.Mode || a.OtherThan != b.OtherThan {
		return false
	}
	return slices.Equal(a.Namespaces, b.Namespaces)
}

// wildcardAllowsNamespace reports whether w admits namespace id.
func wildcardAllowsNamespace(w wildcardRuntime, ns namespaceID) bool {
	switch w.Mode {
	case WildcardAny:
		return true
	case WildcardOther:
		return ns != emptyNamespaceID && ns != w.OtherThan
	case WildcardLocal:
		return ns == emptyNamespaceID
	case WildcardTargetNamespace:
		return len(w.Namespaces) != 0 && w.Namespaces[0] == ns
	case WildcardList:
		return slices.Contains(w.Namespaces, ns)
	default:
		return false
	}
}

// wildcardSubset reports whether derived admits only namespaces admitted by
// base and is no less strict in processContents.
func wildcardSubset(derived, base wildcardRuntime) bool {
	return derived.Process <= base.Process && wildcardNamespaceSubset(derived, base)
}

func wildcardNamespaceSubset(derived, base wildcardRuntime) bool {
	switch derived.Mode {
	case WildcardAny:
		return base.Mode == WildcardAny
	case WildcardOther:
		return base.Mode == WildcardAny || (base.Mode == WildcardOther && base.OtherThan == derived.OtherThan)
	case WildcardLocal:
		return wildcardAllowsNamespace(base, emptyNamespaceID)
	case WildcardTargetNamespace:
		return len(derived.Namespaces) != 0 && wildcardAllowsNamespace(base, derived.Namespaces[0])
	case WildcardList:
		for _, ns := range derived.Namespaces {
			if !wildcardAllowsNamespace(base, ns) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// wildcardsOverlap reports whether two wildcards can admit the same namespace.
func wildcardsOverlap(a, b wildcardRuntime) bool {
	if a.Mode == WildcardList && len(a.Namespaces) == 0 ||
		b.Mode == WildcardList && len(b.Namespaces) == 0 {
		return false
	}
	if wildcardsAlwaysOverlap(a, b) {
		return true
	}
	if a.Mode == WildcardOther {
		return finiteWildcardsOverlap(b, a)
	}
	if b.Mode == WildcardOther {
		return finiteWildcardsOverlap(a, b)
	}
	if overlap, handled := localWildcardOverlap(a, b); handled {
		return overlap
	}
	return finiteWildcardsOverlap(a, b)
}

func wildcardsAlwaysOverlap(a, b wildcardRuntime) bool {
	return a.Mode == WildcardAny || b.Mode == WildcardAny ||
		a.Mode == WildcardOther && b.Mode == WildcardOther
}

func localWildcardOverlap(a, b wildcardRuntime) (overlap, handled bool) {
	if a.Mode == WildcardLocal {
		return wildcardAllowsNamespace(b, emptyNamespaceID), true
	}
	if b.Mode == WildcardLocal {
		return wildcardAllowsNamespace(a, emptyNamespaceID), true
	}
	return false, false
}

func finiteWildcardsOverlap(a, b wildcardRuntime) bool {
	for _, ns := range wildcardNamespaces(a) {
		if wildcardAllowsNamespace(b, ns) {
			return true
		}
	}
	return false
}

// unionWildcard returns the union of two wildcard namespace sets.
func unionWildcard(wa, wb wildcardRuntime, process ProcessContents) (wildcardRuntime, error) {
	if wildcardNamespaceEqual(wa, wb) {
		out := cloneWildcard(wa)
		out.Process = process
		return out, nil
	}
	if wa.Mode == WildcardAny || wb.Mode == WildcardAny {
		return wildcardRuntime{Mode: WildcardAny, Process: process}, nil
	}
	if wa.Mode == WildcardOther && wb.Mode == WildcardOther {
		return wildcardRuntime{Mode: WildcardOther, OtherThan: emptyNamespaceID, Process: process}, nil
	}
	if wa.Mode == WildcardOther {
		return unionOtherWithFinite(wa, wb, process)
	}
	if wb.Mode == WildcardOther {
		return unionOtherWithFinite(wb, wa, process)
	}
	namespaces := append(wildcardFiniteNamespaces(wa), wildcardFiniteNamespaces(wb)...)
	namespaces = normalizeNamespaceList(namespaces)
	return wildcardRuntime{Mode: WildcardList, Namespaces: namespaces, Process: process}, nil
}

// intersectWildcard returns the intersection of two wildcard namespace sets.
func intersectWildcard(wa, wb wildcardRuntime, process ProcessContents) (wildcardRuntime, error) {
	if wildcardNamespaceEqual(wa, wb) {
		out := cloneWildcard(wa)
		out.Process = process
		return out, nil
	}
	if wa.Mode == WildcardAny {
		out := cloneWildcard(wb)
		out.Process = process
		return out, nil
	}
	if wb.Mode == WildcardAny {
		out := cloneWildcard(wa)
		out.Process = process
		return out, nil
	}
	if wa.Mode == WildcardOther && wb.Mode == WildcardOther {
		return intersectOtherWildcards(wa, wb, process)
	}
	return intersectFiniteWildcards(wa, wb, process), nil
}

func intersectOtherWildcards(wa, wb wildcardRuntime, process ProcessContents) (wildcardRuntime, error) {
	if wa.OtherThan == emptyNamespaceID {
		out := cloneWildcard(wb)
		out.Process = process
		return out, nil
	}
	if wb.OtherThan == emptyNamespaceID {
		out := cloneWildcard(wa)
		out.Process = process
		return out, nil
	}
	return wildcardRuntime{}, errors.New("attribute wildcard intersection is not expressible")
}

func intersectFiniteWildcards(wa, wb wildcardRuntime, process ProcessContents) wildcardRuntime {
	candidates := append(wildcardFiniteNamespaces(wa), wildcardFiniteNamespaces(wb)...)
	candidates = normalizeNamespaceList(candidates)
	var namespaces []namespaceID
	for _, ns := range candidates {
		if wildcardAllowsNamespace(wa, ns) && wildcardAllowsNamespace(wb, ns) {
			namespaces = append(namespaces, ns)
		}
	}
	return wildcardRuntime{Mode: WildcardList, Namespaces: namespaces, Process: process}
}

func unionOtherWithFinite(other, finite wildcardRuntime, process ProcessContents) (wildcardRuntime, error) {
	namespaces := wildcardFiniteNamespaces(finite)
	hasAbsent := slices.Contains(namespaces, emptyNamespaceID)
	hasNegated := slices.Contains(namespaces, other.OtherThan)
	if other.OtherThan == emptyNamespaceID {
		if hasAbsent {
			return wildcardRuntime{Mode: WildcardAny, Process: process}, nil
		}
		return wildcardRuntime{Mode: WildcardOther, OtherThan: other.OtherThan, Process: process}, nil
	}
	switch {
	case hasAbsent && hasNegated:
		return wildcardRuntime{Mode: WildcardAny, Process: process}, nil
	case hasNegated:
		return wildcardRuntime{Mode: WildcardOther, OtherThan: emptyNamespaceID, Process: process}, nil
	case hasAbsent:
		return wildcardRuntime{}, errors.New("attribute wildcard union is not expressible")
	default:
		return wildcardRuntime{Mode: WildcardOther, OtherThan: other.OtherThan, Process: process}, nil
	}
}

func wildcardFiniteNamespaces(w wildcardRuntime) []namespaceID {
	switch w.Mode {
	case WildcardLocal:
		return []namespaceID{emptyNamespaceID}
	case WildcardTargetNamespace, WildcardList:
		return slices.Clone(w.Namespaces)
	case WildcardAny, WildcardOther:
		return nil
	default:
	}
	return nil
}

func wildcardNamespaces(w wildcardRuntime) []namespaceID {
	if w.Mode == WildcardTargetNamespace || w.Mode == WildcardList {
		return w.Namespaces
	}
	return nil
}
