package diag

import (
	"cmp"
	"fmt"
	"iter"
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
)

// Severity defines diagnostic severity.
type Severity uint8

const (
	// SeverityError marks a hard validation or compile error.
	SeverityError Severity = iota
	// SeverityWarning marks a non-fatal diagnostic.
	SeverityWarning
)

// Diagnostic carries normalized diagnostic metadata.
type Diagnostic struct {
	Code     string
	Message  string
	Path     string
	Document string
	Actual   string
	Expected []string
	Line     int
	Column   int
	Severity Severity
}

// Collector stores diagnostics in insertion order.
type Collector struct {
	items []Diagnostic
}

// NewCollector creates a collector with optional preallocation.
func NewCollector(capacity int) *Collector {
	if capacity < 0 {
		capacity = 0
	}
	return &Collector{items: make([]Diagnostic, 0, capacity)}
}

// Add appends one diagnostic.
func (c *Collector) Add(d Diagnostic) {
	if c == nil {
		return
	}
	if len(d.Expected) != 0 {
		d.Expected = slices.Clone(d.Expected)
	}
	c.items = append(c.items, d)
}

// Addf appends one error-severity diagnostic with formatted message.
func (c *Collector) Addf(code xsderrors.ErrorCode, path, format string, args ...any) {
	c.Add(Diagnostic{
		Code:     string(code),
		Message:  fmt.Sprintf(format, args...),
		Path:     path,
		Severity: SeverityError,
	})
}

// Len returns diagnostic count.
func (c *Collector) Len() int {
	if c == nil {
		return 0
	}
	return len(c.items)
}

// Seq yields diagnostics in insertion order.
func (c *Collector) Seq() iter.Seq[Diagnostic] {
	return func(yield func(Diagnostic) bool) {
		if c == nil {
			return
		}
		for i := range c.items {
			if !yield(c.items[i]) {
				return
			}
		}
	}
}

// Sorted returns a deterministically sorted diagnostic copy.
func (c *Collector) Sorted() []Diagnostic {
	if c == nil || len(c.items) == 0 {
		return nil
	}
	out := make([]Diagnostic, len(c.items))
	copy(out, c.items)
	slices.SortStableFunc(out, compareDiagnostic)
	return out
}

// ToValidationList converts diagnostics to validation errors.
func (c *Collector) ToValidationList() xsderrors.ValidationList {
	if c == nil || len(c.items) == 0 {
		return nil
	}
	sorted := c.Sorted()
	out := make(xsderrors.ValidationList, 0, len(sorted))
	for i := range sorted {
		item := &sorted[i]
		out = append(out, xsderrors.Validation{
			Code:     item.Code,
			Message:  item.Message,
			Document: item.Document,
			Path:     item.Path,
			Actual:   item.Actual,
			Expected: slices.Clone(item.Expected),
			Line:     item.Line,
			Column:   item.Column,
		})
	}
	return out
}

// ErrorOrNil returns nil when no diagnostics exist, otherwise a ValidationList.
func (c *Collector) ErrorOrNil() error {
	list := c.ToValidationList()
	if len(list) == 0 {
		return nil
	}
	return list
}

func compareDiagnostic(a, b Diagnostic) int {
	if a.Document == "" && b.Document != "" {
		return 1
	}
	if a.Document != "" && b.Document == "" {
		return -1
	}
	if a.Document != b.Document {
		return cmp.Compare(a.Document, b.Document)
	}
	lineA := max(a.Line, 0)
	lineB := max(b.Line, 0)
	if lineA != lineB {
		return cmp.Compare(lineA, lineB)
	}
	colA := max(a.Column, 0)
	colB := max(b.Column, 0)
	if colA != colB {
		return cmp.Compare(colA, colB)
	}
	if a.Code != b.Code {
		return cmp.Compare(a.Code, b.Code)
	}
	return cmp.Compare(a.Message, b.Message)
}
