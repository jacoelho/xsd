package runtime

import (
	"errors"
	"slices"
)

// WildcardMode identifies the namespace set represented by a wildcard.
type WildcardMode uint8

const (
	// WildcardAny allows every namespace.
	WildcardAny WildcardMode = iota
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
	// ProcessStrict requires a declaration for wildcard matches.
	ProcessStrict ProcessContents = iota
	// ProcessLax validates wildcard matches when a declaration is available.
	ProcessLax
	// ProcessSkip accepts wildcard matches without declaration validation.
	ProcessSkip
)

// Wildcard is the runtime representation of an element or attribute wildcard.
type Wildcard struct {
	Namespaces []NamespaceID
	OtherThan  NamespaceID
	Mode       WildcardMode
	Process    ProcessContents
}

// WildcardByID resolves and clones a wildcard from a wildcard table.
func WildcardByID(wildcards []Wildcard, id WildcardID) (Wildcard, bool) {
	if !ValidWildcardID(id, len(wildcards)) {
		return Wildcard{}, false
	}
	return CloneWildcard(wildcards[id]), true
}

// WildcardView is a read-only validation view over a frozen wildcard.
type WildcardView struct {
	otherThan  string
	namespaces []string
	mode       WildcardMode
	process    ProcessContents
	valid      bool
}

// Mode returns the namespace constraint mode.
func (v WildcardView) Mode() WildcardMode {
	return v.mode
}

// NewWildcardView returns a read-only validation view over wildcard.
func NewWildcardView(names *NameTable, wildcard *Wildcard) WildcardView {
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
		if names == nil || !names.ValidNamespaceID(wildcard.OtherThan) {
			view.valid = false
			return view
		}
		view.otherThan = names.Namespace(wildcard.OtherThan)
		return view
	case WildcardTargetNamespace, WildcardList:
		if names == nil {
			view.valid = false
			return view
		}
		view.namespaces = make([]string, len(wildcard.Namespaces))
		for i, ns := range wildcard.Namespaces {
			if !names.ValidNamespaceID(ns) {
				view.valid = false
				return view
			}
			view.namespaces[i] = names.Namespace(ns)
		}
		return view
	default:
		view.valid = false
		return view
	}
}

// NewWildcardViews returns read-only validation views over wildcards.
func NewWildcardViews(names *NameTable, wildcards []Wildcard) []WildcardView {
	out := make([]WildcardView, len(wildcards))
	for i := range wildcards {
		out[i] = NewWildcardView(names, &wildcards[i])
	}
	return out
}

// Process reports the wildcard processContents mode.
func (v WildcardView) Process() ProcessContents {
	if !v.valid {
		return ProcessStrict
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

// EqualWildcardViews reports whether two validation wildcard views expose the
// same wildcard process and namespace set.
func EqualWildcardViews(a, b WildcardView) bool {
	return a.otherThan == b.otherThan &&
		a.mode == b.mode &&
		a.process == b.process &&
		a.valid == b.valid &&
		slices.Equal(a.namespaces, b.namespaces)
}

// EqualWildcardViewProjection reports whether view matches the validation view
// derived from wildcard.
func EqualWildcardViewProjection(view WildcardView, names *NameTable, wildcard *Wildcard) bool {
	return EqualWildcardViews(view, NewWildcardView(names, wildcard))
}

// EqualWildcardViewProjectionTable reports whether views match validation views
// derived from wildcards.
func EqualWildcardViewProjectionTable(views []WildcardView, names *NameTable, wildcards []Wildcard) bool {
	if len(views) != len(wildcards) {
		return false
	}
	for i := range views {
		if !EqualWildcardViewProjection(views[i], names, &wildcards[i]) {
			return false
		}
	}
	return true
}

// ValidateWildcardViewProjectionTable validates wildcard read projections
// against frozen wildcard records.
func ValidateWildcardViewProjectionTable(views []WildcardView, names *NameTable, wildcards []Wildcard) error {
	if len(views) != len(wildcards) {
		return errors.New("wildcard read projection count does not match wildcards")
	}
	if !EqualWildcardViewProjectionTable(views, names, wildcards) {
		return errors.New("wildcard read projection does not match wildcard")
	}
	return nil
}

// WildcardViewByID returns a validation wildcard view from the frozen wildcard
// read projection table.
func WildcardViewByID(views []WildcardView, id WildcardID) (WildcardView, bool) {
	if !ValidWildcardID(id, len(views)) {
		return WildcardView{}, false
	}
	return views[id], true
}

// ValidateWildcard validates frozen wildcard metadata.
func ValidateWildcard(names *NameTable, w Wildcard) error {
	if names == nil {
		return errors.New("wildcard requires name table")
	}
	switch w.Process {
	case ProcessStrict, ProcessLax, ProcessSkip:
	default:
		return errors.New("wildcard has invalid process contents")
	}
	switch w.Mode {
	case WildcardAny, WildcardLocal:
		if len(w.Namespaces) != 0 || w.OtherThan != EmptyNamespaceID {
			return errors.New("wildcard stores inactive namespace fields")
		}
	case WildcardOther:
		if len(w.Namespaces) != 0 || !names.ValidNamespaceID(w.OtherThan) {
			return errors.New("wildcard other namespace is invalid")
		}
	case WildcardTargetNamespace:
		if len(w.Namespaces) != 1 || w.OtherThan != EmptyNamespaceID || !names.ValidNamespaceID(w.Namespaces[0]) {
			return errors.New("targetNamespace wildcard has invalid namespace")
		}
	case WildcardList:
		if w.OtherThan != EmptyNamespaceID {
			return errors.New("namespace list wildcard stores inactive other namespace")
		}
		if !validWildcardNamespaceList(names, w.Namespaces) {
			return errors.New("namespace list wildcard is invalid")
		}
	default:
		return errors.New("wildcard has invalid mode")
	}
	return nil
}

func validWildcardNamespaceList(names *NameTable, namespaces []NamespaceID) bool {
	var prev NamespaceID
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

// NormalizeNamespaceList sorts namespaces and removes duplicate IDs.
func NormalizeNamespaceList(namespaces []NamespaceID) []NamespaceID {
	slices.Sort(namespaces)
	return slices.Compact(namespaces)
}

// WildcardNamespaceEqual reports whether two wildcards represent the same
// namespace set, ignoring process contents.
func WildcardNamespaceEqual(a, b Wildcard) bool {
	if a.Mode != b.Mode || a.OtherThan != b.OtherThan {
		return false
	}
	return slices.Equal(a.Namespaces, b.Namespaces)
}

// WildcardAllowsNamespace reports whether w admits namespace id.
func WildcardAllowsNamespace(w Wildcard, ns NamespaceID) bool {
	switch w.Mode {
	case WildcardAny:
		return true
	case WildcardOther:
		return ns != EmptyNamespaceID && ns != w.OtherThan
	case WildcardLocal:
		return ns == EmptyNamespaceID
	case WildcardTargetNamespace:
		return len(w.Namespaces) != 0 && w.Namespaces[0] == ns
	case WildcardList:
		return slices.Contains(w.Namespaces, ns)
	default:
		return false
	}
}

// WildcardAllowsURI reports whether w admits namespace uri, including URIs not
// interned in names.
func WildcardAllowsURI(names *NameTable, w Wildcard, uri string) bool {
	if w.Mode == WildcardAny {
		return true
	}
	if names == nil {
		return false
	}
	id, ok := names.LookupNamespace(uri)
	if !ok {
		return w.Mode == WildcardOther
	}
	return WildcardAllowsNamespace(w, id)
}

// WildcardSubset reports whether derived admits only namespaces admitted by
// base and is no less strict in processContents.
func WildcardSubset(derived, base Wildcard) bool {
	if derived.Process > base.Process {
		return false
	}
	switch derived.Mode {
	case WildcardAny:
		return base.Mode == WildcardAny
	case WildcardOther:
		return base.Mode == WildcardAny || (base.Mode == WildcardOther && base.OtherThan == derived.OtherThan)
	case WildcardLocal:
		return WildcardAllowsNamespace(base, EmptyNamespaceID)
	case WildcardTargetNamespace:
		return len(derived.Namespaces) != 0 && WildcardAllowsNamespace(base, derived.Namespaces[0])
	case WildcardList:
		for _, ns := range derived.Namespaces {
			if !WildcardAllowsNamespace(base, ns) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// WildcardsOverlap reports whether two wildcards can admit the same namespace.
func WildcardsOverlap(a, b Wildcard) bool {
	if a.Mode == WildcardAny || b.Mode == WildcardAny {
		return true
	}
	if a.Mode == WildcardOther && b.Mode == WildcardOther {
		return true
	}
	if a.Mode == WildcardOther {
		return wildcardHasNamespaceOtherThan(b, a.OtherThan)
	}
	if b.Mode == WildcardOther {
		return wildcardHasNamespaceOtherThan(a, b.OtherThan)
	}
	if a.Mode == WildcardLocal && b.Mode == WildcardLocal {
		return true
	}
	if a.Mode == WildcardLocal {
		return WildcardAllowsNamespace(b, EmptyNamespaceID)
	}
	if b.Mode == WildcardLocal {
		return WildcardAllowsNamespace(a, EmptyNamespaceID)
	}
	for _, ns := range wildcardNamespaces(a) {
		if WildcardAllowsNamespace(b, ns) {
			return true
		}
	}
	return false
}

// UnionWildcard returns the union of two wildcard namespace sets.
func UnionWildcard(wa, wb Wildcard, process ProcessContents) (Wildcard, error) {
	if WildcardNamespaceEqual(wa, wb) {
		out := CloneWildcard(wa)
		out.Process = process
		return out, nil
	}
	if wa.Mode == WildcardAny || wb.Mode == WildcardAny {
		return Wildcard{Mode: WildcardAny, Process: process}, nil
	}
	if wa.Mode == WildcardOther && wb.Mode == WildcardOther {
		return Wildcard{Mode: WildcardOther, OtherThan: EmptyNamespaceID, Process: process}, nil
	}
	if wa.Mode == WildcardOther {
		return unionOtherWithFinite(wa, wb, process)
	}
	if wb.Mode == WildcardOther {
		return unionOtherWithFinite(wb, wa, process)
	}
	namespaces := append(wildcardFiniteNamespaces(wa), wildcardFiniteNamespaces(wb)...)
	namespaces = NormalizeNamespaceList(namespaces)
	return Wildcard{Mode: WildcardList, Namespaces: namespaces, Process: process}, nil
}

// IntersectWildcard returns the intersection of two wildcard namespace sets.
func IntersectWildcard(wa, wb Wildcard, process ProcessContents) (Wildcard, error) {
	if WildcardNamespaceEqual(wa, wb) {
		out := CloneWildcard(wa)
		out.Process = process
		return out, nil
	}
	if wa.Mode == WildcardAny {
		out := CloneWildcard(wb)
		out.Process = process
		return out, nil
	}
	if wb.Mode == WildcardAny {
		out := CloneWildcard(wa)
		out.Process = process
		return out, nil
	}
	if wa.Mode == WildcardOther && wb.Mode == WildcardOther {
		if wa.OtherThan == EmptyNamespaceID {
			out := CloneWildcard(wb)
			out.Process = process
			return out, nil
		}
		if wb.OtherThan == EmptyNamespaceID {
			out := CloneWildcard(wa)
			out.Process = process
			return out, nil
		}
		return Wildcard{}, errors.New("attribute wildcard intersection is not expressible")
	}
	candidates := append(wildcardFiniteNamespaces(wa), wildcardFiniteNamespaces(wb)...)
	candidates = NormalizeNamespaceList(candidates)
	var namespaces []NamespaceID
	for _, ns := range candidates {
		if WildcardAllowsNamespace(wa, ns) && WildcardAllowsNamespace(wb, ns) {
			namespaces = append(namespaces, ns)
		}
	}
	return Wildcard{Mode: WildcardList, Namespaces: namespaces, Process: process}, nil
}

func unionOtherWithFinite(other, finite Wildcard, process ProcessContents) (Wildcard, error) {
	namespaces := wildcardFiniteNamespaces(finite)
	hasAbsent := slices.Contains(namespaces, EmptyNamespaceID)
	hasNegated := slices.Contains(namespaces, other.OtherThan)
	if other.OtherThan == EmptyNamespaceID {
		if hasAbsent {
			return Wildcard{Mode: WildcardAny, Process: process}, nil
		}
		return Wildcard{Mode: WildcardOther, OtherThan: other.OtherThan, Process: process}, nil
	}
	switch {
	case hasAbsent && hasNegated:
		return Wildcard{Mode: WildcardAny, Process: process}, nil
	case hasNegated:
		return Wildcard{Mode: WildcardOther, OtherThan: EmptyNamespaceID, Process: process}, nil
	case hasAbsent:
		return Wildcard{}, errors.New("attribute wildcard union is not expressible")
	default:
		return Wildcard{Mode: WildcardOther, OtherThan: other.OtherThan, Process: process}, nil
	}
}

func wildcardFiniteNamespaces(w Wildcard) []NamespaceID {
	switch w.Mode {
	case WildcardLocal:
		return []NamespaceID{EmptyNamespaceID}
	case WildcardTargetNamespace, WildcardList:
		return slices.Clone(w.Namespaces)
	default:
		return nil
	}
}

func wildcardHasNamespaceOtherThan(w Wildcard, excluded NamespaceID) bool {
	switch w.Mode {
	case WildcardAny, WildcardOther:
		return true
	case WildcardLocal:
		return false
	case WildcardTargetNamespace, WildcardList:
		for _, ns := range wildcardNamespaces(w) {
			if ns != excluded {
				return true
			}
		}
	}
	return false
}

func wildcardNamespaces(w Wildcard) []NamespaceID {
	if w.Mode == WildcardTargetNamespace || w.Mode == WildcardList {
		return w.Namespaces
	}
	return nil
}
