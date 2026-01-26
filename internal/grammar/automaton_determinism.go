package grammar

import (
	"fmt"
	"strings"
)

// SymbolOverlapFunc reports whether two symbols can match the same element.
type SymbolOverlapFunc func(a, b *Symbol) bool

// CheckDeterminism reports a UPA violation if a state contains ambiguous symbols or overlapping matches.
func (a *Automaton) CheckDeterminism(overlap SymbolOverlapFunc) error {
	if a == nil || overlap == nil {
		return nil
	}
	for _, row := range a.stateSymbolPos {
		if len(row) == 0 {
			continue
		}
		for i, pos := range row {
			if pos == symbolPosAmbiguous {
				return fmt.Errorf("content model is not deterministic: symbol %s is ambiguous", a.symbols[i])
			}
		}
		for i := 0; i < len(row); i++ {
			if row[i] == symbolPosNone {
				continue
			}
			for j := i + 1; j < len(row); j++ {
				if row[j] == symbolPosNone {
					continue
				}
				if overlap(&a.symbols[i], &a.symbols[j]) {
					return fmt.Errorf("content model is not deterministic: symbols %s and %s overlap", a.symbols[i], a.symbols[j])
				}
			}
		}
	}
	return nil
}

func (s Symbol) String() string {
	switch s.Kind {
	case KindElement:
		return s.QName.String()
	case KindAny:
		return "##any"
	case KindAnyNS:
		if s.NS == "" {
			return "##local"
		}
		return "##namespace(" + s.NS + ")"
	case KindAnyOther:
		if s.NS == "" {
			return "##notAbsent"
		}
		return "##other(" + s.NS + ")"
	case KindAnyNSList:
		if len(s.NSList) == 0 {
			return "##list()"
		}
		parts := make([]string, 0, len(s.NSList))
		for _, ns := range s.NSList {
			parts = append(parts, ns.String())
		}
		return "##list(" + strings.Join(parts, ",") + ")"
	default:
		return "symbol"
	}
}
