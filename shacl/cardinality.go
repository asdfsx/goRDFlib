package shacl

import (
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

// parseInt parses an integer from an RDF term.
// Returns 0 if the value is not a valid integer.
func parseInt(t Term) int {
	v, err := strconv.Atoi(t.Value())
	if err != nil {
		return 0
	}
	return v
}
