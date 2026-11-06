package value

import (
	"errors"
	"math"
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

func compilePattern(t *testing.T, source string) *xsdregex.Pattern {
	t.Helper()
	pattern, err := xsdregex.Compile(source)
	if err != nil {
		t.Fatal(err)
	}
	return pattern
}

// patternWork returns the work xsdregex reports for one match.
func patternWork(t *testing.T, pattern *xsdregex.Pattern, input string) uint64 {
	t.Helper()
	_, work, err := pattern.MatchStringWithScratch(input, xsdregex.MatchOptions{MaxWork: 1 << 30}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return work
}

func patternBudget(t *testing.T, patternWork uint64) *evaluationBudget {
	t.Helper()
	budget, err := newEvaluationBudget(EvaluationLimits{Work: 1 << 30, PatternWork: patternWork, PatternTotal: testPatternTotal()})
	if err != nil {
		t.Fatal(err)
	}
	return &budget
}

// applyPatterns runs both the string and the raw-bytes path against fresh
// copies of the same budget state and returns one error per path.
func applyPatterns(t *testing.T, groups [][]*Pattern, input string, limit uint64) [2]error {
	t.Helper()
	var scratch Scratch
	stringBudget, bytesBudget := patternBudget(t, limit), patternBudget(t, limit)
	return [2]error{
		applyPatternFacets(groups, input, &scratch, stringBudget),
		applyPatternFacetsBytes(groups, []byte(input), &scratch, bytesBudget),
	}
}

func TestPatternWorkLimitStopsBeforeAlternatives(t *testing.T) {
	first := compilePattern(t, `(a|aa)*b`)
	alternative := compilePattern(t, `aab`)
	w1 := patternWork(t, first, "aab")
	groups := [][]*Pattern{{first, alternative}}
	for path, err := range applyPatterns(t, groups, "aab", w1) {
		if err != nil {
			t.Fatalf("path=%d at w1: %v", path, err)
		}
	}
	for path, err := range applyPatterns(t, groups, "aab", w1-1) {
		if !errors.Is(err, ErrPatternLimit) || !errors.Is(err, ErrLimit) {
			t.Fatalf("path=%d at w1-1: %v, want ErrPatternLimit wrapping ErrLimit before the alternative", path, err)
		}
	}
}

func TestPatternWorkChargesAlternativesAfterFirstFailure(t *testing.T) {
	first := compilePattern(t, `x+`)
	alternative := compilePattern(t, `[a-z]+`)
	w1 := patternWork(t, first, "abc")
	w2 := patternWork(t, alternative, "abc")
	groups := [][]*Pattern{{first, alternative}}
	for path, err := range applyPatterns(t, groups, "abc", w1+w2) {
		if err != nil {
			t.Fatalf("path=%d at w1+w2: %v", path, err)
		}
	}
	for path, err := range applyPatterns(t, groups, "abc", w1+w2-1) {
		if !errors.Is(err, ErrPatternLimit) {
			t.Fatalf("path=%d at w1+w2-1: %v, want ErrPatternLimit", path, err)
		}
	}
}

func TestPatternGroupsShareOneBudget(t *testing.T) {
	a := compilePattern(t, `[a-z]+`)
	b := compilePattern(t, `.*`)
	w := patternWork(t, a, "abc") + patternWork(t, b, "abc")
	groups := [][]*Pattern{{a}, {b}}
	for path, err := range applyPatterns(t, groups, "abc", w) {
		if err != nil {
			t.Fatalf("path=%d at sum: %v", path, err)
		}
	}
	for path, err := range applyPatterns(t, groups, "abc", w-1) {
		if !errors.Is(err, ErrPatternLimit) {
			t.Fatalf("path=%d at sum-1: %v, want ErrPatternLimit", path, err)
		}
	}
}

func TestEvaluationLimitsRequireEveryField(t *testing.T) {
	for _, limits := range []EvaluationLimits{{}, {Work: 1}, {PatternWork: 1}} {
		if _, err := newEvaluationBudget(limits); !errors.Is(err, ErrMetadata) {
			t.Errorf("newEvaluationBudget(%+v) error = %v, want ErrMetadata", limits, err)
		}
	}
}

func TestProgramValidatePatternWorkIsPerValue(t *testing.T) {
	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 20, MaxTotalPatternWork: 1 << 62})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if completeErr := builder.Complete(id, TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString, Base: BuiltinType(PrimitiveString), ListItem: NoType,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Facets: FacetSpec{Patterns: [][]*Pattern{{compilePattern(t, `[a-z]+`)}}},
	}); completeErr != nil {
		t.Fatal(completeErr)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	const input = "abcdef"
	work := patternWork(t, compilePattern(t, `[a-z]+`), input)
	for _, test := range []struct {
		name  string
		limit uint64
		ok    bool
	}{{"at", work, true}, {"below", work - 1, false}} {
		limits := EvaluationLimits{Work: 1 << 20, PatternWork: test.limit, PatternTotal: testPatternTotal()}
		_, strErr := program.Validate(id, input, Resolver{}, 0, limits, nil)
		_, rawErr := program.ValidateBytes(id, []byte(input), Resolver{}, 0, limits, nil)
		for _, err := range []error{strErr, rawErr} {
			if test.ok != (err == nil) || (!test.ok && !errors.Is(err, ErrPatternLimit)) {
				t.Errorf("%s: error = %v, want ok=%v", test.name, err, test.ok)
			}
		}
	}
}

func TestPatternAccountAttribution(t *testing.T) {
	pattern := compilePattern(t, `[a-z]+`)
	const input = "abcdef"
	cost := patternWork(t, pattern, input)
	if cost < 4 {
		t.Fatalf("cost = %d, too small for the table", cost)
	}
	tests := []struct {
		name                  string
		perLimit, perUsed     uint64
		totalLimit, totalUsed uint64
		want                  error
	}{
		{"total remainder smaller", 100 * cost, 0, cost, 1, ErrPatternTotalLimit},
		{"equal remainders", cost, 1, cost + 1, 2, ErrPatternLimit},
		{"per-value remainder smaller", cost - 1, 0, 100 * cost, 0, ErrPatternLimit},
		{"both remainders zero", cost, cost, cost, cost, ErrPatternLimit},
		{"total remainder zero", 100 * cost, 0, cost, cost, ErrPatternTotalLimit},
	}
	for _, tc := range tests {
		for _, path := range []string{"string", "bytes"} {
			t.Run(tc.name+"/"+path, func(t *testing.T) {
				var account PatternAccount
				account.Reset(tc.totalLimit)
				account.used = tc.totalUsed
				budget := evaluationBudget{limit: 1 << 30, patternLimit: tc.perLimit, patternUsed: tc.perUsed, total: &account}
				var err error
				if path == "string" {
					_, err = budget.matchString(pattern, input, nil)
				} else {
					_, err = budget.matchBytes(pattern, []byte(input), nil)
				}
				if !errors.Is(err, tc.want) || !errors.Is(err, ErrLimit) {
					t.Fatalf("error = %v, want %v wrapping ErrLimit", err, tc.want)
				}
				if errors.Is(tc.want, ErrPatternTotalLimit) && errors.Is(err, ErrPatternLimit) {
					t.Fatalf("total failure also matches ErrPatternLimit")
				}
				if errors.Is(tc.want, ErrPatternLimit) && errors.Is(err, ErrPatternTotalLimit) {
					t.Fatalf("per-value failure also matches ErrPatternTotalLimit")
				}
			})
		}
	}
}

func TestPatternAccountChargesOnlyGrantedWork(t *testing.T) {
	pattern := compilePattern(t, `[a-z]+`)
	cost := patternWork(t, pattern, "abcdef")
	small := patternWork(t, pattern, "a")
	var account PatternAccount
	account.Reset(100 * cost)
	budget := evaluationBudget{limit: 1 << 30, patternLimit: small, total: &account}
	if _, err := budget.matchString(pattern, "abcdef", nil); !errors.Is(err, ErrPatternLimit) {
		t.Fatalf("error = %v, want ErrPatternLimit", err)
	}
	if account.used != small {
		t.Fatalf("total used = %d, want only the granted %d", account.used, small)
	}
	next := evaluationBudget{limit: 1 << 30, patternLimit: 100 * cost, total: &account}
	if ok, err := next.matchString(pattern, "a", nil); err != nil || !ok {
		t.Fatalf("follow-up match = %v, %v", ok, err)
	}
}

func TestPatternAccountExhaustionPersists(t *testing.T) {
	pattern := compilePattern(t, `[a-z]+`)
	cost := patternWork(t, pattern, "abcdef")
	var account PatternAccount
	account.Reset(cost - 1)
	for range 2 {
		budget := evaluationBudget{limit: 1 << 30, patternLimit: 100 * cost, total: &account}
		if _, err := budget.matchString(pattern, "abcdef", nil); !errors.Is(err, ErrPatternTotalLimit) {
			t.Fatalf("error = %v, want ErrPatternTotalLimit", err)
		}
	}
}

func TestPatternAccountSaturates(t *testing.T) {
	pattern := compilePattern(t, `[a-z]+`)
	var account PatternAccount
	account.Reset(math.MaxUint64)
	budget := evaluationBudget{limit: 1 << 30, patternLimit: math.MaxUint64, total: &account}
	for range 3 {
		if ok, err := budget.matchString(pattern, "abc", nil); err != nil || !ok {
			t.Fatalf("match = %v, %v", ok, err)
		}
	}
	account.used = math.MaxUint64 - 1
	budget.chargePattern(10)
	if account.used != math.MaxUint64 {
		t.Fatalf("used = %d, want saturation", account.used)
	}
}

func TestEvaluationRequiresPatternAccount(t *testing.T) {
	program, err := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 20, MaxTotalPatternWork: 1 << 20}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	var zero PatternAccount
	var funded PatternAccount
	funded.Reset(1)
	for name, limits := range map[string]EvaluationLimits{
		"nil":  {Work: 1 << 20, PatternWork: 1},
		"zero": {Work: 1 << 20, PatternWork: 1, PatternTotal: &zero},
	} {
		if _, err := program.Validate(builtinString, "x", Resolver{}, 0, limits, nil); !errors.Is(err, ErrMetadata) {
			t.Errorf("%s: error = %v, want ErrMetadata", name, err)
		}
	}
	if _, err := program.Validate(builtinString, "x", Resolver{}, 0, EvaluationLimits{Work: 1 << 20, PatternWork: 1, PatternTotal: &funded}, nil); err != nil {
		t.Errorf("funded account: %v", err)
	}
}

func TestBuilderWithoutTotalPatternWorkRejectsLiterals(t *testing.T) {
	builder := NewBuilder(BuilderOptions{MaxPatternWork: 1 << 20})
	if _, err := builder.Validate(builtinString, "x", Resolver{}, 0, nil); !errors.Is(err, ErrMetadata) {
		t.Fatalf("error = %v, want ErrMetadata", err)
	}
}
