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

// wildcardByID resolves a wildcard from a wildcard table. The result shares
// the admitted namespace list, which is immutable.
func wildcardByID(wildcards []wildcardRuntime, id WildcardID) (wildcardRuntime, bool) {
	if !validWildcardID(id, len(wildcards)) {
		return wildcardRuntime{}, false
	}
	return wildcards[id], true
}

// WildcardView is a read-only validation view over a frozen wildcard. Its
// namespaces are in text order, the order diagnostics list them in and
// AllowsURI binary-searches.
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
	sortNamespaceURIs(view.namespaces)
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
		_, found := slices.BinarySearch(v.namespaces, uri)
		return found
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

// Admitted namespace lists are immutable, sorted and strictly increasing
// (validWildcardNamespaceList), so readers share them and set operations look
// namespaces up by binary search or merge two lists. An operation charges spend
// before it visits a list: one step per namespace it looks up or merges.
// Overlap and intersection look up the shorter of the two runs that lie inside
// the range the other list spans, since no namespace outside it can match;
// subset looks up each namespace of the subset list; union merges both lists;
// intersection with ##other copies the finite list; equality compares lists
// of equal length. Union and intersection of two lists of the same mode
// charge no separate equality check: their result has the length of both
// lists exactly when the lists are equal. Membership of one namespace is one
// binary search, and a list compared with ##other, which rejects at most two
// namespaces, visits at most three; callers do these once per item they have
// already charged, a name, a particle pair or an attribute use.

// wildcardNamespaceEqual reports whether two wildcards represent the same
// namespace set, ignoring process contents.
func wildcardNamespaceEqual(a, b wildcardRuntime, spend func(int) error) (bool, error) {
	if a.Mode != b.Mode || a.OtherThan != b.OtherThan || len(a.Namespaces) != len(b.Namespaces) {
		return false, nil
	}
	if err := spend(len(a.Namespaces)); err != nil {
		return false, err
	}
	return slices.Equal(a.Namespaces, b.Namespaces), nil
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
	case WildcardTargetNamespace, WildcardList:
		return containsNamespace(w.Namespaces, ns)
	default:
		return false
	}
}

// wildcardSubset reports whether derived admits only namespaces admitted by
// base and is no less strict in processContents.
func wildcardSubset(derived, base wildcardRuntime, spend func(int) error) (bool, error) {
	if derived.Process > base.Process {
		return false, nil
	}
	return wildcardNamespaceSubset(derived, base, spend)
}

func wildcardNamespaceSubset(derived, base wildcardRuntime, spend func(int) error) (bool, error) {
	switch derived.Mode {
	case WildcardAny:
		return base.Mode == WildcardAny, nil
	case WildcardOther:
		return base.Mode == WildcardAny || (base.Mode == WildcardOther && base.OtherThan == derived.OtherThan), nil
	case WildcardLocal:
		return wildcardAllowsNamespace(base, emptyNamespaceID), nil
	case WildcardTargetNamespace:
		return len(derived.Namespaces) != 0 && wildcardAllowsNamespace(base, derived.Namespaces[0]), nil
	case WildcardList:
		return wildcardAllowsNamespaces(base, derived.Namespaces, spend)
	default:
		return false, nil
	}
}

func wildcardAllowsNamespaces(w wildcardRuntime, namespaces []namespaceID, spend func(int) error) (bool, error) {
	switch w.Mode {
	case WildcardAny:
		return true, nil
	case WildcardOther:
		return !containsNamespace(namespaces, emptyNamespaceID) && !containsNamespace(namespaces, w.OtherThan), nil
	case WildcardLocal:
		return len(namespaces) == 0 || len(namespaces) == 1 && namespaces[0] == emptyNamespaceID, nil
	case WildcardTargetNamespace, WildcardList:
		return sortedNamespaceSubset(namespaces, w.Namespaces, spend)
	default:
		return len(namespaces) == 0, nil
	}
}

// wildcardsOverlap reports whether two wildcards can admit the same namespace.
func wildcardsOverlap(a, b wildcardRuntime, spend func(int) error) (bool, error) {
	if a.Mode == WildcardList && len(a.Namespaces) == 0 ||
		b.Mode == WildcardList && len(b.Namespaces) == 0 {
		return false, nil
	}
	if wildcardsAlwaysOverlap(a, b) {
		return true, nil
	}
	if a.Mode == WildcardOther {
		return otherAllowsAnyNamespace(a, wildcardFiniteNamespaces(b)), nil
	}
	if b.Mode == WildcardOther {
		return otherAllowsAnyNamespace(b, wildcardFiniteNamespaces(a)), nil
	}
	if overlap, handled := localWildcardOverlap(a, b); handled {
		return overlap, nil
	}
	return sortedNamespacesOverlap(a.Namespaces, b.Namespaces, spend)
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

func otherAllowsAnyNamespace(other wildcardRuntime, namespaces []namespaceID) bool {
	for _, ns := range namespaces {
		if wildcardAllowsNamespace(other, ns) {
			return true
		}
	}
	return false
}

// unionWildcard returns the union of two wildcard namespace sets.
func unionWildcard(wa, wb wildcardRuntime, process ProcessContents, spend func(int) error) (wildcardRuntime, error) {
	switch {
	case sameNamespaceListMode(wa, wb):
		namespaces, err := mergeNamespaceLists(wa.Namespaces, wb.Namespaces, spend)
		if err != nil {
			return wildcardRuntime{}, err
		}
		return namespaceListResult(wa, wb, namespaces, process), nil
	case sameUnlistedNamespaces(wa, wb):
		wa.Process = process
		return wa, nil
	case wa.Mode == WildcardAny || wb.Mode == WildcardAny:
		return wildcardRuntime{Mode: WildcardAny, Process: process}, nil
	case wa.Mode == WildcardOther && wb.Mode == WildcardOther:
		return wildcardRuntime{Mode: WildcardOther, OtherThan: emptyNamespaceID, Process: process}, nil
	case wa.Mode == WildcardOther:
		return unionOtherWithFinite(wa, wb, process)
	case wb.Mode == WildcardOther:
		return unionOtherWithFinite(wb, wa, process)
	}
	namespaces, err := mergeNamespaceLists(wildcardFiniteNamespaces(wa), wildcardFiniteNamespaces(wb), spend)
	if err != nil {
		return wildcardRuntime{}, err
	}
	return wildcardRuntime{Mode: WildcardList, Namespaces: namespaces, Process: process}, nil
}

// intersectWildcard returns the intersection of two wildcard namespace sets.
func intersectWildcard(wa, wb wildcardRuntime, process ProcessContents, spend func(int) error) (wildcardRuntime, error) {
	switch {
	case sameNamespaceListMode(wa, wb):
		namespaces, err := intersectNamespaceLists(wa.Namespaces, wb.Namespaces, spend)
		if err != nil {
			return wildcardRuntime{}, err
		}
		return namespaceListResult(wa, wb, namespaces, process), nil
	case sameUnlistedNamespaces(wa, wb), wb.Mode == WildcardAny:
		wa.Process = process
		return wa, nil
	case wa.Mode == WildcardAny:
		wb.Process = process
		return wb, nil
	case wa.Mode == WildcardOther && wb.Mode == WildcardOther:
		return intersectOtherWildcards(wa, wb, process)
	case wa.Mode == WildcardOther:
		return intersectOtherWithFinite(wa, wb, process, spend)
	case wb.Mode == WildcardOther:
		return intersectOtherWithFinite(wb, wa, process, spend)
	}
	namespaces, err := intersectNamespaceLists(wildcardFiniteNamespaces(wa), wildcardFiniteNamespaces(wb), spend)
	if err != nil {
		return wildcardRuntime{}, err
	}
	return wildcardRuntime{Mode: WildcardList, Namespaces: namespaces, Process: process}, nil
}

func sameNamespaceListMode(wa, wb wildcardRuntime) bool {
	return wa.Mode == wb.Mode && (wa.Mode == WildcardTargetNamespace || wa.Mode == WildcardList)
}

func sameUnlistedNamespaces(wa, wb wildcardRuntime) bool {
	return wa.Mode == wb.Mode && wa.OtherThan == wb.OtherThan && len(wa.Namespaces) == 0 && len(wb.Namespaces) == 0
}

func namespaceListResult(wa, wb wildcardRuntime, namespaces []namespaceID, process ProcessContents) wildcardRuntime {
	if len(namespaces) == len(wa.Namespaces) && len(namespaces) == len(wb.Namespaces) {
		wa.Process = process
		return wa
	}
	return wildcardRuntime{Mode: WildcardList, Namespaces: namespaces, Process: process}
}

func intersectOtherWildcards(wa, wb wildcardRuntime, process ProcessContents) (wildcardRuntime, error) {
	if wa.OtherThan == emptyNamespaceID {
		wb.Process = process
		return wb, nil
	}
	if wb.OtherThan == emptyNamespaceID {
		wa.Process = process
		return wa, nil
	}
	return wildcardRuntime{}, errors.New("attribute wildcard intersection is not expressible")
}

func intersectOtherWithFinite(other, finite wildcardRuntime, process ProcessContents, spend func(int) error) (wildcardRuntime, error) {
	candidates := wildcardFiniteNamespaces(finite)
	if err := spend(len(candidates)); err != nil {
		return wildcardRuntime{}, err
	}
	var namespaces []namespaceID
	for _, ns := range candidates {
		if wildcardAllowsNamespace(other, ns) {
			namespaces = append(namespaces, ns)
		}
	}
	return wildcardRuntime{Mode: WildcardList, Namespaces: namespaces, Process: process}, nil
}

func unionOtherWithFinite(other, finite wildcardRuntime, process ProcessContents) (wildcardRuntime, error) {
	namespaces := wildcardFiniteNamespaces(finite)
	hasAbsent := containsNamespace(namespaces, emptyNamespaceID)
	hasNegated := containsNamespace(namespaces, other.OtherThan)
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
		return w.Namespaces
	case WildcardAny, WildcardOther:
		return nil
	default:
	}
	return nil
}

func containsNamespace(namespaces []namespaceID, ns namespaceID) bool {
	_, found := slices.BinarySearch(namespaces, ns)
	return found
}

// namespacesWithin returns the run of sorted namespaces inside the range that
// the sorted list bounds spans.
func namespacesWithin(namespaces, bounds []namespaceID) []namespaceID {
	if len(namespaces) == 0 || len(bounds) == 0 {
		return nil
	}
	start, _ := slices.BinarySearch(namespaces, bounds[0])
	end, found := slices.BinarySearch(namespaces, bounds[len(bounds)-1])
	if found {
		end++
	}
	return namespaces[start:end]
}

// overlappingRuns returns the runs of a and b inside the range the other list
// spans, the shorter first.
func overlappingRuns(a, b []namespaceID) (shorter, longer []namespaceID) {
	runA, runB := namespacesWithin(a, b), namespacesWithin(b, a)
	if len(runA) <= len(runB) {
		return runA, runB
	}
	return runB, runA
}

func sortedNamespacesOverlap(a, b []namespaceID, spend func(int) error) (bool, error) {
	shorter, longer := overlappingRuns(a, b)
	if err := spend(len(shorter)); err != nil {
		return false, err
	}
	for _, ns := range shorter {
		if containsNamespace(longer, ns) {
			return true, nil
		}
	}
	return false, nil
}

func sortedNamespaceSubset(subset, set []namespaceID, spend func(int) error) (bool, error) {
	if err := spend(len(subset)); err != nil {
		return false, err
	}
	for _, ns := range subset {
		if !containsNamespace(set, ns) {
			return false, nil
		}
	}
	return true, nil
}

func intersectNamespaceLists(a, b []namespaceID, spend func(int) error) ([]namespaceID, error) {
	shorter, longer := overlappingRuns(a, b)
	if err := spend(len(shorter)); err != nil {
		return nil, err
	}
	var out []namespaceID
	for _, ns := range shorter {
		if containsNamespace(longer, ns) {
			out = append(out, ns)
		}
	}
	return out, nil
}

func mergeNamespaceLists(a, b []namespaceID, spend func(int) error) ([]namespaceID, error) {
	if err := spend(len(a) + len(b)); err != nil {
		return nil, err
	}
	out := make([]namespaceID, 0, len(a)+len(b))
	for len(a) != 0 && len(b) != 0 {
		switch {
		case a[0] < b[0]:
			out, a = append(out, a[0]), a[1:]
		case a[0] > b[0]:
			out, b = append(out, b[0]), b[1:]
		default:
			out, a, b = append(out, a[0]), a[1:], b[1:]
		}
	}
	out = append(out, a...)
	return append(out, b...), nil
}
