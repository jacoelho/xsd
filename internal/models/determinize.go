package models

import (
	"errors"
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

const defaultMaxDFAStates = 4096

// Limits configures DFA determinization thresholds.
type Limits struct {
	MaxDFAStates uint32
}

// Model holds the compiled content model.
type Model struct {
	DFA  runtime.DFAModel
	NFA  runtime.NFAModel
	Kind runtime.ModelKind
}

var errDFALimitExceeded = errors.New("dfa state limit exceeded")

// Compile determinizes the model or falls back to an NFA when the DFA is too large.
func Compile(glu *Glushkov, matchers []runtime.PosMatcher, limits Limits) (Model, error) {
	if glu == nil {
		return Model{}, fmt.Errorf("glushkov model is nil")
	}
	if len(glu.Positions) == 0 {
		return Model{Kind: runtime.ModelNone}, nil
	}
	if len(matchers) != len(glu.Positions) {
		return Model{}, fmt.Errorf("matcher count %d does not match positions %d", len(matchers), len(glu.Positions))
	}
	maxStates := limits.MaxDFAStates
	if maxStates == 0 {
		maxStates = defaultMaxDFAStates
	}

	dfa, err := determinize(glu, matchers, maxStates)
	if err != nil {
		if errors.Is(err, errDFALimitExceeded) {
			return Model{Kind: runtime.ModelNFA, NFA: buildNFA(glu, matchers)}, nil
		}
		return Model{}, err
	}
	return Model{Kind: runtime.ModelDFA, DFA: dfa}, nil
}

func buildNFA(glu *Glushkov, matchers []runtime.PosMatcher) runtime.NFAModel {
	return runtime.NFAModel{
		Bitsets:   glu.Bitsets,
		Start:     glu.First,
		Accept:    glu.Last,
		Nullable:  glu.Nullable,
		FollowOff: 0,
		FollowLen: uint32(len(glu.Follow)),
		Matchers:  append([]runtime.PosMatcher(nil), matchers...),
		Follow:    append([]runtime.BitsetRef(nil), glu.Follow...),
	}
}

func determinize(glu *Glushkov, matchers []runtime.PosMatcher, maxStates uint32) (runtime.DFAModel, error) {
	if glu.firstRaw == nil || glu.lastRaw == nil || len(glu.followRaw) == 0 {
		return runtime.DFAModel{}, fmt.Errorf("glushkov raw bitsets missing")
	}
	size := len(matchers)
	start := newBitset(size)

	states := []*bitset{start}
	queue := []uint32{0}
	stateIDs := map[string]uint32{start.key(): 0}

	model := runtime.DFAModel{Start: 0}

	for len(queue) > 0 {
		stateID := queue[0]
		queue = queue[1:]
		state := states[stateID]

		reachable := newBitset(size)
		if state.empty() {
			reachable.or(glu.firstRaw)
		} else {
			state.forEach(func(pos int) {
				reachable.or(glu.followRaw[pos])
			})
		}

		symNext := make(map[runtime.SymbolID]*bitset)
		symElem := make(map[runtime.SymbolID]runtime.ElemID)
		wildNext := make(map[runtime.WildcardID]*bitset)

		var scanErr error
		reachable.forEach(func(pos int) {
			if scanErr != nil {
				return
			}
			if pos >= len(matchers) {
				return
			}
			matcher := matchers[pos]
			switch matcher.Kind {
			case runtime.PosExact:
				next := symNext[matcher.Sym]
				if next == nil {
					next = newBitset(size)
					symNext[matcher.Sym] = next
					symElem[matcher.Sym] = matcher.Elem
				} else if elem := symElem[matcher.Sym]; elem != matcher.Elem {
					scanErr = fmt.Errorf("symbol %d maps to multiple elements (%d, %d)", matcher.Sym, elem, matcher.Elem)
					return
				}
				next.set(pos)
			case runtime.PosWildcard:
				next := wildNext[matcher.Rule]
				if next == nil {
					next = newBitset(size)
					wildNext[matcher.Rule] = next
				}
				next.set(pos)
			default:
				scanErr = fmt.Errorf("unknown matcher kind %d", matcher.Kind)
				return
			}
		})
		if scanErr != nil {
			return runtime.DFAModel{}, scanErr
		}

		stateRecord := runtime.DFAState{Accept: intersects(state, glu.lastRaw) || (state.empty() && glu.Nullable)}
		stateRecord.TransOff = uint32(len(model.Transitions))
		stateRecord.WildOff = uint32(len(model.Wildcards))

		symbols := make([]runtime.SymbolID, 0, len(symNext))
		for sym := range symNext {
			symbols = append(symbols, sym)
		}
		slices.Sort(symbols)
		for _, sym := range symbols {
			next := symNext[sym]
			if next.empty() {
				continue
			}
			nextID, err := getOrCreateState(next, &states, stateIDs, &queue, maxStates)
			if err != nil {
				return runtime.DFAModel{}, err
			}
			model.Transitions = append(model.Transitions, runtime.DFATransition{
				Sym:  sym,
				Next: nextID,
				Elem: symElem[sym],
			})
		}

		wildcards := make([]runtime.WildcardID, 0, len(wildNext))
		for rule := range wildNext {
			wildcards = append(wildcards, rule)
		}
		slices.Sort(wildcards)
		for _, rule := range wildcards {
			next := wildNext[rule]
			if next.empty() {
				continue
			}
			nextID, err := getOrCreateState(next, &states, stateIDs, &queue, maxStates)
			if err != nil {
				return runtime.DFAModel{}, err
			}
			model.Wildcards = append(model.Wildcards, runtime.DFAWildcardEdge{
				Rule: rule,
				Next: nextID,
			})
		}

		stateRecord.TransLen = uint32(len(model.Transitions)) - stateRecord.TransOff
		stateRecord.WildLen = uint32(len(model.Wildcards)) - stateRecord.WildOff
		model.States = append(model.States, stateRecord)
	}

	return model, nil
}

func getOrCreateState(next *bitset, states *[]*bitset, stateIDs map[string]uint32, queue *[]uint32, maxStates uint32) (uint32, error) {
	key := next.key()
	if id, ok := stateIDs[key]; ok {
		return id, nil
	}
	if maxStates > 0 && uint32(len(*states)) >= maxStates {
		return 0, errDFALimitExceeded
	}
	id := uint32(len(*states))
	stateIDs[key] = id
	*states = append(*states, next)
	*queue = append(*queue, id)
	return id, nil
}

func intersects(a, b *bitset) bool {
	if a == nil || b == nil {
		return false
	}
	limit := min(len(b.words), len(a.words))
	for i := range limit {
		if a.words[i]&b.words[i] != 0 {
			return true
		}
	}
	return false
}
