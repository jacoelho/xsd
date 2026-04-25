package xsdpath

import "github.com/jacoelho/xsd/internal/runtime"

type AttributePolicy = runtime.AttributePolicy
type Expression = runtime.Expression
type NodeTest = runtime.NodeTest
type Path = runtime.Path
type Step = runtime.Step

const (
	AttributesAllowed    = runtime.AttributesAllowed
	AttributesDisallowed = runtime.AttributesDisallowed
	AxisChild            = runtime.AxisChild
	AxisDescendantOrSelf = runtime.AxisDescendantOrSelf
	AxisSelf             = runtime.AxisSelf
)

var ErrInvalidXPath = runtime.ErrInvalidXPath

func CanonicalizeNodeTest(test NodeTest) NodeTest {
	return runtime.CanonicalizeNodeTest(test)
}

func Parse(expr string, nsContext map[string]string, policy AttributePolicy) (Expression, error) {
	return runtime.Parse(expr, nsContext, policy)
}
