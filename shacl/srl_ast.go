package shacl

// SRL namespace.
const SRL = "http://www.w3.org/ns/shacl-rules#"

// RuleSet is the top-level AST node for an SRL document.
type RuleSet struct {
	Base       string
	Prefixes   map[string]string
	DataBlocks [][]SRLTriple
	Rules      []SRLRule
}

// SRLRule represents a single RULE ... WHERE ... block.
type SRLRule struct {
	Head []SRLTriple
	Body []SRLBodyElement
}

// SRLTriple is a triple pattern (subject, predicate, object).
type SRLTriple struct {
	Subject   SRLTerm
	Predicate SRLTerm
	Object    SRLTerm
}

// SRLTermKind distinguishes different kinds of SRL terms.
type SRLTermKind int

const (
	SRLTermIRI SRLTermKind = iota
	SRLTermVariable
	SRLTermLiteral
	SRLTermBlankNode
	SRLTermTripleTerm // <<( s p o )>>
)

// SRLTerm represents a term in an SRL document.
type SRLTerm struct {
	Kind     SRLTermKind
	Value    string // IRI value, variable name, literal lexical value, blank node ID
	Datatype string // for literals
	Language string // for literals (may include --dir)

	// For triple terms
	TTSubject   *SRLTerm
	TTPredicate *SRLTerm
	TTObject    *SRLTerm
}

// IsVariable returns true if the term is a variable.
func (t SRLTerm) IsVariable() bool { return t.Kind == SRLTermVariable }

// SRLBodyElementKind distinguishes body element types.
type SRLBodyElementKind int

const (
	SRLBodyTriple SRLBodyElementKind = iota
	SRLBodyFilter
	SRLBodyNot
	SRLBodyBind
)

// SRLBodyElement is a single element in a rule body.
type SRLBodyElement struct {
	Kind SRLBodyElementKind

	// For SRLBodyTriple
	Triple SRLTriple

	// For SRLBodyFilter
	FilterExpr string // raw SPARQL expression text

	// For SRLBodyNot
	NotBody []SRLBodyElement

	// For SRLBodyBind
	BindExpr string // raw SPARQL expression text
	BindVar  string // variable name (without ?)
}
