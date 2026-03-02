package shacl

import (
	"fmt"
	"strconv"

	
)

// MinCountConstraint implements sh:minCount.
type MinCountConstraint struct {
	MinCount int
}

func (c *MinCountConstraint) ComponentIRI() string {
	return SH + "MinCountConstraintComponent"
}

func (c *MinCountConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if len(valueNodes) < c.MinCount {
		return []ValidationResult{makeResult(shape, focusNode, Term{}, c.ComponentIRI())}
	}
	return nil
}

// MaxCountConstraint implements sh:maxCount.
type MaxCountConstraint struct {
	MaxCount int
}

func (c *MaxCountConstraint) ComponentIRI() string {
	return SH + "MaxCountConstraintComponent"
}

func (c *MaxCountConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	if len(valueNodes) > c.MaxCount {
		return []ValidationResult{makeResult(shape, focusNode, Term{}, c.ComponentIRI())}
	}
	return nil
}

// parseInt parses an integer from an RDF term. Panics on invalid values
// since SHACL shape definitions must contain valid integers.
func parseInt(t Term) int {
	v, err := strconv.Atoi(t.Value())
	if err != nil {
		panic(fmt.Sprintf("shacl: invalid integer in shape definition: %q: %v", t.Value(), err))
	}
	return v
}
