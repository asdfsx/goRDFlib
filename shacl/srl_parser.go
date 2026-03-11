package shacl

import (
	"fmt"
	"strings"
)

// ParseSRL parses an SRL document into a RuleSet.
func ParseSRL(input string) (*RuleSet, error) {
	p := &srlParser{
		lexer:    newSRLLexer(input),
		prefixes: make(map[string]string),
		bnodeID:  0,
	}
	return p.parse()
}

type srlParser struct {
	lexer    *srlLexer
	prefixes map[string]string
	bnodeID  int
	peeked   *srlToken
}

func (p *srlParser) peek() (srlToken, error) {
	if p.peeked != nil {
		return *p.peeked, nil
	}
	tok, err := p.lexer.next()
	if err != nil {
		return tok, err
	}
	p.peeked = &tok
	return tok, nil
}

func (p *srlParser) next() (srlToken, error) {
	if p.peeked != nil {
		tok := *p.peeked
		p.peeked = nil
		return tok, nil
	}
	return p.lexer.next()
}

func (p *srlParser) expect(kind srlTokenKind) (srlToken, error) {
	tok, err := p.next()
	if err != nil {
		return tok, err
	}
	if tok.kind != kind {
		return tok, fmt.Errorf("expected token kind %d, got %d (%q) at position %d", kind, tok.kind, tok.val, tok.pos)
	}
	return tok, nil
}

func (p *srlParser) newBNode() string {
	p.bnodeID++
	return fmt.Sprintf("b%d", p.bnodeID)
}

func (p *srlParser) parse() (*RuleSet, error) {
	rs := &RuleSet{Prefixes: make(map[string]string)}
	for {
		tok, err := p.peek()
		if err != nil {
			return nil, err
		}
		switch tok.kind {
		case srlTokEOF:
			rs.Prefixes = p.prefixes
			return rs, nil
		case srlTokPrefix:
			if err := p.parsePrefix(); err != nil {
				return nil, err
			}
		case srlTokBase:
			if err := p.parseBase(); err != nil {
				return nil, err
			}
		case srlTokData:
			data, err := p.parseDataBlock()
			if err != nil {
				return nil, err
			}
			rs.DataBlocks = append(rs.DataBlocks, data)
		case srlTokRule:
			rule, err := p.parseRule()
			if err != nil {
				return nil, err
			}
			rs.Rules = append(rs.Rules, rule)
		default:
			return nil, fmt.Errorf("unexpected token %q at position %d", tok.val, tok.pos)
		}
	}
}

func (p *srlParser) parsePrefix() error {
	p.next() // consume PREFIX
	tok, err := p.next()
	if err != nil {
		return err
	}
	if tok.kind != srlTokPName {
		return fmt.Errorf("expected prefix name, got %q at position %d", tok.val, tok.pos)
	}
	prefix := tok.val
	if !strings.HasSuffix(prefix, ":") {
		return fmt.Errorf("prefix name must end with ':', got %q at position %d", prefix, tok.pos)
	}
	prefix = strings.TrimSuffix(prefix, ":")

	iriTok, err := p.expect(srlTokIRI)
	if err != nil {
		return err
	}
	p.prefixes[prefix] = iriTok.val
	return nil
}

func (p *srlParser) parseBase() error {
	p.next() // consume BASE
	iriTok, err := p.expect(srlTokIRI)
	if err != nil {
		return err
	}
	p.lexer.base = iriTok.val
	return nil
}

func (p *srlParser) parseDataBlock() ([]SRLTriple, error) {
	p.next() // consume DATA
	if _, err := p.expect(srlTokLBrace); err != nil {
		return nil, err
	}
	triples, err := p.parseTriplePatterns(false)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(srlTokRBrace); err != nil {
		return nil, err
	}
	// Validate: DATA blocks cannot contain variables
	for _, t := range triples {
		if t.Subject.IsVariable() || t.Predicate.IsVariable() || t.Object.IsVariable() {
			return nil, fmt.Errorf("variables are not allowed in DATA blocks")
		}
	}
	return triples, nil
}

func (p *srlParser) parseRule() (SRLRule, error) {
	p.next() // consume RULE
	if _, err := p.expect(srlTokLBrace); err != nil {
		return SRLRule{}, err
	}
	head, err := p.parseTriplePatterns(true)
	if err != nil {
		return SRLRule{}, err
	}
	if _, err := p.expect(srlTokRBrace); err != nil {
		return SRLRule{}, err
	}
	if _, err := p.expect(srlTokWhere); err != nil {
		return SRLRule{}, err
	}
	if _, err := p.expect(srlTokLBrace); err != nil {
		return SRLRule{}, err
	}
	body, err := p.parseBodyElements()
	if err != nil {
		return SRLRule{}, err
	}
	if _, err := p.expect(srlTokRBrace); err != nil {
		return SRLRule{}, err
	}
	return SRLRule{Head: head, Body: body}, nil
}

// parseTriplePatterns parses triple patterns inside { }.
// allowVars controls whether variables are permitted.
func (p *srlParser) parseTriplePatterns(allowVars bool) ([]SRLTriple, error) {
	var triples []SRLTriple
	for {
		tok, err := p.peek()
		if err != nil {
			return nil, err
		}
		if tok.kind == srlTokRBrace {
			return triples, nil
		}
		// Parse subject
		subj, subjTriples, err := p.parseTerm(allowVars)
		if err != nil {
			return nil, err
		}
		triples = append(triples, subjTriples...)

		// Handle reified triple shorthand: << s p o >> at top level
		tok, err = p.peek()
		if err != nil {
			return nil, err
		}

		// Parse predicate-object list
		poTriples, err := p.parsePredicateObjectList(subj, allowVars)
		if err != nil {
			return nil, err
		}
		triples = append(triples, poTriples...)

		// Optional dot
		tok, err = p.peek()
		if err != nil {
			return nil, err
		}
		if tok.kind == srlTokDot {
			p.next()
		}
	}
}

func (p *srlParser) parsePredicateObjectList(subj SRLTerm, allowVars bool) ([]SRLTriple, error) {
	var triples []SRLTriple

	tok, err := p.peek()
	if err != nil {
		return nil, err
	}
	// Empty predicate-object list (reified triple shorthand)
	if tok.kind == srlTokRBrace || tok.kind == srlTokDot {
		return triples, nil
	}

	for {
		pred, predTriples, err := p.parsePredicate(allowVars)
		if err != nil {
			return nil, err
		}
		triples = append(triples, predTriples...)

		// Parse object list
		for {
			obj, objTriples, err := p.parseTerm(allowVars)
			if err != nil {
				return nil, err
			}
			triples = append(triples, objTriples...)
			triples = append(triples, SRLTriple{Subject: subj, Predicate: pred, Object: obj})

			// Check for annotation {| ... |}
			tok, err := p.peek()
			if err != nil {
				return nil, err
			}
			if tok.kind == srlTokTilde || tok.kind == srlTokLAnnot {
				annTriples, err := p.parseAnnotations(subj, pred, obj, allowVars)
				if err != nil {
					return nil, err
				}
				triples = append(triples, annTriples...)
			}

			// Comma = more objects
			tok, err = p.peek()
			if err != nil {
				return nil, err
			}
			if tok.kind == srlTokComma {
				p.next()
				continue
			}
			break
		}

		// Semicolon = more predicates
		tok, err = p.peek()
		if err != nil {
			return nil, err
		}
		if tok.kind == srlTokSemicolon {
			p.next()
			// Check if next is } or . (trailing semicolon)
			tok, err = p.peek()
			if err != nil {
				return nil, err
			}
			if tok.kind == srlTokRBrace || tok.kind == srlTokDot {
				break
			}
			continue
		}
		break
	}
	return triples, nil
}

func (p *srlParser) parsePredicate(allowVars bool) (SRLTerm, []SRLTriple, error) {
	tok, err := p.peek()
	if err != nil {
		return SRLTerm{}, nil, err
	}
	if tok.kind == srlTokA {
		p.next()
		return SRLTerm{Kind: SRLTermIRI, Value: RDFType}, nil, nil
	}
	// Blank nodes are not allowed as predicates.
	if tok.kind == srlTokBNode || tok.kind == srlTokLBrack {
		return SRLTerm{}, nil, fmt.Errorf("blank nodes not allowed as predicates at position %d", tok.pos)
	}
	return p.parseTerm(allowVars)
}

func (p *srlParser) parseTerm(allowVars bool) (SRLTerm, []SRLTriple, error) {
	tok, err := p.peek()
	if err != nil {
		return SRLTerm{}, nil, err
	}

	switch tok.kind {
	case srlTokIRI:
		p.next()
		return SRLTerm{Kind: SRLTermIRI, Value: tok.val}, nil, nil
	case srlTokPName:
		p.next()
		iri, err := p.expandPName(tok.val)
		if err != nil {
			return SRLTerm{}, nil, err
		}
		return SRLTerm{Kind: SRLTermIRI, Value: iri}, nil, nil
	case srlTokVar:
		if !allowVars {
			return SRLTerm{}, nil, fmt.Errorf("variables not allowed here at position %d", tok.pos)
		}
		p.next()
		return SRLTerm{Kind: SRLTermVariable, Value: tok.val}, nil, nil
	case srlTokBNode:
		p.next()
		return SRLTerm{Kind: SRLTermBlankNode, Value: tok.val}, nil, nil
	case srlTokLBrack:
		return p.parseBlankNodePropertyList(allowVars)
	case srlTokString:
		return p.parseLiteral()
	case srlTokInteger:
		p.next()
		return SRLTerm{Kind: SRLTermLiteral, Value: tok.val, Datatype: XSD + "integer"}, nil, nil
	case srlTokDecimal:
		p.next()
		return SRLTerm{Kind: SRLTermLiteral, Value: tok.val, Datatype: XSD + "decimal"}, nil, nil
	case srlTokDouble:
		p.next()
		return SRLTerm{Kind: SRLTermLiteral, Value: tok.val, Datatype: XSD + "double"}, nil, nil
	case srlTokTrue:
		p.next()
		return SRLTerm{Kind: SRLTermLiteral, Value: "true", Datatype: XSD + "boolean"}, nil, nil
	case srlTokFalse:
		p.next()
		return SRLTerm{Kind: SRLTermLiteral, Value: "false", Datatype: XSD + "boolean"}, nil, nil
	case srlTokA:
		p.next()
		return SRLTerm{Kind: SRLTermIRI, Value: RDFType}, nil, nil
	case srlTokLParen:
		return p.parseCollection(allowVars)
	case srlTokLTriple:
		return p.parseTripleTerm(allowVars)
	}
	return SRLTerm{}, nil, fmt.Errorf("unexpected token %q (kind %d) at position %d", tok.val, tok.kind, tok.pos)
}

func (p *srlParser) parseBlankNodePropertyList(allowVars bool) (SRLTerm, []SRLTriple, error) {
	p.next() // consume [
	bnode := SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}

	tok, err := p.peek()
	if err != nil {
		return SRLTerm{}, nil, err
	}
	if tok.kind == srlTokRBrack {
		p.next()
		return bnode, nil, nil
	}

	triples, err := p.parsePredicateObjectList(bnode, allowVars)
	if err != nil {
		return SRLTerm{}, nil, err
	}
	if _, err := p.expect(srlTokRBrack); err != nil {
		return SRLTerm{}, nil, err
	}
	return bnode, triples, nil
}

func (p *srlParser) parseLiteral() (SRLTerm, []SRLTriple, error) {
	strTok, err := p.next()
	if err != nil {
		return SRLTerm{}, nil, err
	}

	t := SRLTerm{Kind: SRLTermLiteral, Value: strTok.val}

	tok, err := p.peek()
	if err != nil {
		return SRLTerm{}, nil, err
	}

	if tok.kind == srlTokHat {
		p.next() // consume ^^
		dtTok, err := p.next()
		if err != nil {
			return SRLTerm{}, nil, err
		}
		switch dtTok.kind {
		case srlTokIRI:
			t.Datatype = dtTok.val
		case srlTokPName:
			iri, err := p.expandPName(dtTok.val)
			if err != nil {
				return SRLTerm{}, nil, err
			}
			t.Datatype = iri
		default:
			return SRLTerm{}, nil, fmt.Errorf("expected IRI after ^^, got %q at position %d", dtTok.val, dtTok.pos)
		}
	} else if tok.kind == srlTokAt {
		p.next() // consume @
		// Read language tag — manually advance lexer past peeked token.
		p.peeked = nil
		langStart := p.lexer.pos
		for p.lexer.pos < len(p.lexer.input) {
			c := p.lexer.input[p.lexer.pos]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
				p.lexer.pos++
			} else {
				break
			}
		}
		lang := p.lexer.input[langStart:p.lexer.pos]
		t.Language = lang
		t.Datatype = RDF + "langString"
		// Check for directional tag --dir
		if p.lexer.pos+2 < len(p.lexer.input) && p.lexer.input[p.lexer.pos] == '-' && p.lexer.input[p.lexer.pos+1] == '-' {
			p.lexer.pos += 2
			dirStart := p.lexer.pos
			for p.lexer.pos < len(p.lexer.input) {
				c := p.lexer.input[p.lexer.pos]
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
					p.lexer.pos++
				} else {
					break
				}
			}
			dir := p.lexer.input[dirStart:p.lexer.pos]
			t.Language = lang + "--" + dir
			t.Datatype = RDF + "dirLangString"
		}
	} else {
		t.Datatype = XSD + "string"
	}
	return t, nil, nil
}

func (p *srlParser) parseCollection(allowVars bool) (SRLTerm, []SRLTriple, error) {
	p.next() // consume (

	var items []SRLTerm
	var triples []SRLTriple
	for {
		tok, err := p.peek()
		if err != nil {
			return SRLTerm{}, nil, err
		}
		if tok.kind == srlTokRParen {
			p.next()
			break
		}
		item, itemTriples, err := p.parseTerm(allowVars)
		if err != nil {
			return SRLTerm{}, nil, err
		}
		items = append(items, item)
		triples = append(triples, itemTriples...)
	}

	if len(items) == 0 {
		return SRLTerm{Kind: SRLTermIRI, Value: RDFNil}, triples, nil
	}

	// Build RDF list
	first := SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}
	current := first
	rdfFirst := SRLTerm{Kind: SRLTermIRI, Value: RDFFirst}
	rdfRest := SRLTerm{Kind: SRLTermIRI, Value: RDFRest}
	rdfNil := SRLTerm{Kind: SRLTermIRI, Value: RDFNil}

	for i, item := range items {
		triples = append(triples, SRLTriple{Subject: current, Predicate: rdfFirst, Object: item})
		if i < len(items)-1 {
			next := SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}
			triples = append(triples, SRLTriple{Subject: current, Predicate: rdfRest, Object: next})
			current = next
		} else {
			triples = append(triples, SRLTriple{Subject: current, Predicate: rdfRest, Object: rdfNil})
		}
	}
	return first, triples, nil
}

func (p *srlParser) parseTripleTerm(allowVars bool) (SRLTerm, []SRLTriple, error) {
	tok, _ := p.next() // consume << or <<(

	s, sTriples, err := p.parseTerm(allowVars)
	if err != nil {
		return SRLTerm{}, nil, err
	}
	pred, pTriples, err := p.parsePredicate(allowVars)
	if err != nil {
		return SRLTerm{}, nil, err
	}
	obj, oTriples, err := p.parseTerm(allowVars)
	if err != nil {
		return SRLTerm{}, nil, err
	}

	var allTriples []SRLTriple
	allTriples = append(allTriples, sTriples...)
	allTriples = append(allTriples, pTriples...)
	allTriples = append(allTriples, oTriples...)

	// Check for reifier ~name
	var reifier *SRLTerm
	next, err := p.peek()
	if err != nil {
		return SRLTerm{}, nil, err
	}
	if next.kind == srlTokTilde {
		p.next()
		reifTerm, rTriples, err := p.parseTerm(allowVars)
		if err != nil {
			return SRLTerm{}, nil, err
		}
		reifier = &reifTerm
		allTriples = append(allTriples, rTriples...)
	}

	// Consume >> or )>>
	closeTok, err := p.next()
	if err != nil {
		return SRLTerm{}, nil, err
	}
	if closeTok.kind != srlTokRTriple {
		return SRLTerm{}, nil, fmt.Errorf("expected >> or )>>, got %q at position %d", closeTok.val, closeTok.pos)
	}

	tt := SRLTerm{
		Kind:        SRLTermTripleTerm,
		TTSubject:   &s,
		TTPredicate: &pred,
		TTObject:    &obj,
	}

	// If reifier, generate rdf:reifies triple
	if reifier != nil {
		rdfReifies := SRLTerm{Kind: SRLTermIRI, Value: RDF + "reifies"}
		allTriples = append(allTriples, SRLTriple{Subject: *reifier, Predicate: rdfReifies, Object: tt})
	}

	// For reified triple shorthand at top level: << s p o >> generates
	// a bnode reifier + rdf:reifies triple + the inner triple
	if tok.val == "<<" && reifier == nil {
		bn := SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}
		rdfReifies := SRLTerm{Kind: SRLTermIRI, Value: RDF + "reifies"}
		allTriples = append(allTriples, SRLTriple{Subject: bn, Predicate: rdfReifies, Object: tt})
		allTriples = append(allTriples, SRLTriple{Subject: s, Predicate: pred, Object: obj})

		// Check for annotations
		next, err := p.peek()
		if err != nil {
			return SRLTerm{}, nil, err
		}
		if next.kind == srlTokLAnnot {
			annTriples, err := p.parseAnnotationBlock(bn, allowVars)
			if err != nil {
				return SRLTerm{}, nil, err
			}
			allTriples = append(allTriples, annTriples...)
		}

		return bn, allTriples, nil
	}

	return tt, allTriples, nil
}

func (p *srlParser) parseAnnotations(subj, pred, obj SRLTerm, allowVars bool) ([]SRLTriple, error) {
	var triples []SRLTriple
	for {
		tok, err := p.peek()
		if err != nil {
			return nil, err
		}

		var reifier SRLTerm
		if tok.kind == srlTokTilde {
			p.next()
			reifTerm, rTriples, err := p.parseTerm(allowVars)
			if err != nil {
				return nil, err
			}
			reifier = reifTerm
			triples = append(triples, rTriples...)
		} else {
			reifier = SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}
		}

		// Generate rdf:reifies
		tt := SRLTerm{Kind: SRLTermTripleTerm, TTSubject: &subj, TTPredicate: &pred, TTObject: &obj}
		rdfReifies := SRLTerm{Kind: SRLTermIRI, Value: RDF + "reifies"}
		triples = append(triples, SRLTriple{Subject: reifier, Predicate: rdfReifies, Object: tt})

		tok, err = p.peek()
		if err != nil {
			return nil, err
		}
		if tok.kind == srlTokLAnnot {
			annTriples, err := p.parseAnnotationBlock(reifier, allowVars)
			if err != nil {
				return nil, err
			}
			triples = append(triples, annTriples...)
		}

		// Check for more annotations
		tok, err = p.peek()
		if err != nil {
			return nil, err
		}
		if tok.kind != srlTokTilde && tok.kind != srlTokLAnnot {
			break
		}
	}
	return triples, nil
}

func (p *srlParser) parseAnnotationBlock(reifier SRLTerm, allowVars bool) ([]SRLTriple, error) {
	p.next() // consume {|
	triples, err := p.parsePredicateObjectList(reifier, allowVars)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(srlTokRAnnot); err != nil {
		return nil, err
	}
	return triples, nil
}

func (p *srlParser) parseBodyElements() ([]SRLBodyElement, error) {
	var elements []SRLBodyElement
	for {
		tok, err := p.peek()
		if err != nil {
			return nil, err
		}
		if tok.kind == srlTokRBrace {
			return elements, nil
		}

		switch tok.kind {
		case srlTokFilter:
			p.next()
			expr, err := p.lexer.scanExpr()
			if err != nil {
				return nil, err
			}
			p.peeked = nil // clear peeked after manual scanning
			elements = append(elements, SRLBodyElement{Kind: SRLBodyFilter, FilterExpr: expr})
		case srlTokBind:
			p.next()
			expr, err := p.lexer.scanExpr()
			if err != nil {
				return nil, err
			}
			p.peeked = nil
			// Parse "expr AS ?var" from the expression text
			bindExpr, bindVar, err := parseBind(expr)
			if err != nil {
				return nil, err
			}
			elements = append(elements, SRLBodyElement{Kind: SRLBodyBind, BindExpr: bindExpr, BindVar: bindVar})
		case srlTokNot:
			p.next()
			if _, err := p.expect(srlTokLBrace); err != nil {
				return nil, err
			}
			notBody, err := p.parseBodyElements()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(srlTokRBrace); err != nil {
				return nil, err
			}
			elements = append(elements, SRLBodyElement{Kind: SRLBodyNot, NotBody: notBody})
		default:
			// Triple pattern — parse subject
			subj, subjTriples, err := p.parseTerm(true)
			if err != nil {
				return nil, err
			}
			// Add any auxiliary triples from collections/blank nodes as body triples
			for _, t := range subjTriples {
				elements = append(elements, SRLBodyElement{Kind: SRLBodyTriple, Triple: t})
			}

			// Parse predicate-object list
			poTriples, err := p.parsePredicateObjectList(subj, true)
			if err != nil {
				return nil, err
			}
			for _, t := range poTriples {
				elements = append(elements, SRLBodyElement{Kind: SRLBodyTriple, Triple: t})
			}

			// Optional dot
			tok, err = p.peek()
			if err != nil {
				return nil, err
			}
			if tok.kind == srlTokDot {
				p.next()
			}
		}
	}
}

func (p *srlParser) expandPName(pname string) (string, error) {
	idx := strings.Index(pname, ":")
	if idx < 0 {
		return "", fmt.Errorf("invalid prefixed name %q", pname)
	}
	prefix := pname[:idx]
	local := pname[idx+1:]
	ns, ok := p.prefixes[prefix]
	if !ok {
		return "", fmt.Errorf("undefined prefix %q", prefix)
	}
	return ns + local, nil
}

// parseBind parses "expr AS ?var" from a BIND expression.
func parseBind(expr string) (string, string, error) {
	// Find last "AS" keyword
	upper := strings.ToUpper(expr)
	idx := strings.LastIndex(upper, " AS ")
	if idx < 0 {
		return "", "", fmt.Errorf("BIND expression must contain AS: %q", expr)
	}
	bindExpr := strings.TrimSpace(expr[:idx])
	varPart := strings.TrimSpace(expr[idx+4:])
	if !strings.HasPrefix(varPart, "?") {
		return "", "", fmt.Errorf("expected variable after AS: %q", varPart)
	}
	return bindExpr, varPart[1:], nil
}
