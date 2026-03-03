package turtle

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Parse reads Turtle from r and adds triples to g.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	parser := &turtleParser{
		g:        g,
		input:    string(data),
		base:     cfg.base,
		prefixes: make(map[string]string),
	}
	// Copy graph namespace bindings as initial prefixes
	g.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
		parser.prefixes[prefix] = ns.Value()
		return true
	})
	return parser.parse()
}

type turtleParser struct {
	g        *rdflibgo.Graph
	input    string
	pos      int
	line     int
	col      int
	base     string
	prefixes map[string]string // prefix -> namespace URI
}

// parse is the main entry point.
func (p *turtleParser) parse() error {
	p.line = 1
	p.col = 1
	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}
		if err := p.statement(); err != nil {
			return err
		}
	}
	return nil
}

// statement parses a directive or triple statement.
func (p *turtleParser) statement() error {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil
	}

	ch := p.input[p.pos]

	// @prefix
	if ch == '@' {
		return p.directive()
	}

	// SPARQL-style PREFIX/BASE (case-insensitive)
	if (ch == 'P' || ch == 'p') && p.matchKeywordCI("PREFIX") {
		return p.sparqlPrefix()
	}
	if (ch == 'B' || ch == 'b') && p.matchKeywordCI("BASE") {
		return p.sparqlBase()
	}

	// SPARQL-style VERSION (case-insensitive)
	if (ch == 'V' || ch == 'v') && p.matchKeywordCI("VERSION") {
		return p.sparqlVersion()
	}

	// Triple or reified triple
	return p.tripleStatement()
}

// directive handles @prefix and @base.
func (p *turtleParser) directive() error {
	p.pos++ // skip '@'
	if p.startsWith("prefix") {
		p.pos += 6
		p.skipWS()
		prefix := p.readPrefixName()
		if !p.expect(':') {
			return p.errorf("expected ':' after prefix name")
		}
		p.skipWS()
		iri, err := p.readIRI()
		if err != nil {
			return err
		}
		iri = p.resolveIRI(iri)
		p.prefixes[prefix] = iri
		p.g.Bind(prefix, rdflibgo.NewURIRefUnsafe(iri))
		p.skipWS()
		if !p.expect('.') {
			return p.errorf("expected '.' after @prefix")
		}
		return nil
	}
	if p.startsWith("version") {
		p.pos += 7
		p.skipWS()
		if _, err := p.readVersionString(); err != nil {
			return err
		}
		p.skipWS()
		if !p.expect('.') {
			return p.errorf("expected '.' after @version")
		}
		return nil
	}
	if p.startsWith("base") {
		p.pos += 4
		p.skipWS()
		iri, err := p.readIRI()
		if err != nil {
			return err
		}
		p.base = p.resolveIRI(iri)
		p.skipWS()
		if !p.expect('.') {
			return p.errorf("expected '.' after @base")
		}
		return nil
	}
	return p.errorf("unknown directive")
}

func (p *turtleParser) sparqlPrefix() error {
	p.pos += 6
	p.skipWS()
	prefix := p.readPrefixName()
	if !p.expect(':') {
		return p.errorf("expected ':' after PREFIX name")
	}
	p.skipWS()
	iri, err := p.readIRI()
	if err != nil {
		return err
	}
	iri = p.resolveIRI(iri)
	p.prefixes[prefix] = iri
	p.g.Bind(prefix, rdflibgo.NewURIRefUnsafe(iri))
	return nil
}

func (p *turtleParser) sparqlVersion() error {
	p.pos += 7 // skip "VERSION"
	p.skipWS()
	if _, err := p.readVersionString(); err != nil {
		return err
	}
	return nil
}

// readVersionString reads a single-quoted or double-quoted short string (no triple-quoted).
func (p *turtleParser) readVersionString() (string, error) {
	if p.pos >= len(p.input) {
		return "", p.errorf("expected version string")
	}
	ch := p.input[p.pos]
	if ch != '"' && ch != '\'' {
		return "", p.errorf("expected quoted string for version, got %q", ch)
	}
	// Reject triple-quoted strings
	if p.pos+2 < len(p.input) && p.input[p.pos+1] == ch && p.input[p.pos+2] == ch {
		return "", p.errorf("triple-quoted strings not allowed for version")
	}
	p.pos++ // skip opening quote
	start := p.pos
	for p.pos < len(p.input) {
		if p.input[p.pos] == ch {
			val := p.input[start:p.pos]
			p.pos++
			return val, nil
		}
		if p.input[p.pos] == '\n' || p.input[p.pos] == '\r' {
			return "", p.errorf("newline in version string")
		}
		p.pos++
	}
	return "", p.errorf("unterminated version string")
}

func (p *turtleParser) sparqlBase() error {
	p.pos += 4
	p.skipWS()
	iri, err := p.readIRI()
	if err != nil {
		return err
	}
	p.base = p.resolveIRI(iri)
	return nil
}

// tripleStatement parses: subject predicateObjectList '.'
// Per the Turtle grammar, when the subject is a blankNodePropertyList,
// the predicateObjectList is optional.
// In Turtle 1.2, standalone reified triples << s p o >> . are also valid.
func (p *turtleParser) tripleStatement() error {
	subj, err := p.readSubject()
	if err != nil {
		return err
	}

	p.skipWS()
	// When the subject is a blank node property list [...] or a reified triple,
	// the predicateObjectList is optional — may be followed by just '.'.
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		if _, isBNode := subj.(rdflibgo.BNode); isBNode {
			p.pos++
			return nil
		}
	}

	if err := p.predicateObjectList(subj); err != nil {
		return err
	}

	p.skipWS()
	if !p.expect('.') {
		return p.errorf("expected '.' at end of triple")
	}
	return nil
}

// predicateObjectList parses: verb objectList (';' verb objectList)*
func (p *turtleParser) predicateObjectList(subj rdflibgo.Subject) error {
	pred, err := p.readVerb()
	if err != nil {
		return err
	}

	if err := p.objectList(subj, pred); err != nil {
		return err
	}

	for {
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] != ';' {
			break
		}
		// Consume one or more consecutive semicolons.
		for p.pos < len(p.input) && p.input[p.pos] == ';' {
			p.pos++
			p.skipWS()
		}
		// Allow trailing ';' before '.', ']', or '|}'
		if p.pos >= len(p.input) || p.input[p.pos] == '.' || p.input[p.pos] == ']' || p.input[p.pos] == '|' {
			break
		}
		pred, err = p.readVerb()
		if err != nil {
			return err
		}
		if err := p.objectList(subj, pred); err != nil {
			return err
		}
	}
	return nil
}

// objectList parses: object (',' object)*
// In Turtle 1.2, each object may be followed by reifiers (~id) and annotation blocks ({| ... |}).
func (p *turtleParser) objectList(subj rdflibgo.Subject, pred rdflibgo.URIRef) error {
	obj, err := p.readObject()
	if err != nil {
		return err
	}
	p.g.Add(subj, pred, obj)
	if err := p.readAnnotationsAndReifiers(subj, pred, obj); err != nil {
		return err
	}

	for {
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] != ',' {
			break
		}
		p.pos++ // skip ','
		p.skipWS()
		obj, err = p.readObject()
		if err != nil {
			return err
		}
		p.g.Add(subj, pred, obj)
		if err := p.readAnnotationsAndReifiers(subj, pred, obj); err != nil {
			return err
		}
	}
	return nil
}

// readAnnotationsAndReifiers parses zero or more reifier (~id) and annotation ({| ... |}) blocks
// after a triple's object. Each ~id and/or {| |} creates a reifier node linked via rdf:reifies.
func (p *turtleParser) readAnnotationsAndReifiers(subj rdflibgo.Subject, pred rdflibgo.URIRef, obj rdflibgo.Term) error {
	tt := rdflibgo.NewTripleTerm(subj, pred, obj)
	reifiesPred := rdflibgo.RDFReifies

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}

		// Reifier: ~ id or ~ (anonymous)
		if p.input[p.pos] == '~' {
			p.pos++ // skip ~
			p.skipWS()

			var reifier rdflibgo.Subject
			if p.pos < len(p.input) && p.input[p.pos] != '{' && p.input[p.pos] != '.' &&
				p.input[p.pos] != ';' && p.input[p.pos] != ',' && p.input[p.pos] != ']' &&
				p.input[p.pos] != '~' && p.input[p.pos] != '|' {
				// Named reifier (IRI, prefixed name, or blank node)
				var err error
				reifier, err = p.readReifierID()
				if err != nil {
					return err
				}
			} else {
				// Anonymous reifier
				reifier = rdflibgo.NewBNode()
			}
			p.g.Add(reifier, reifiesPred, tt)

			// Check for annotation block after reifier
			p.skipWS()
			if p.pos+1 < len(p.input) && p.input[p.pos] == '{' && p.input[p.pos+1] == '|' {
				if err := p.readAnnotationBlock(reifier, tt); err != nil {
					return err
				}
			}
			continue
		}

		// Annotation block: {| predObjectList |}
		if p.input[p.pos] == '{' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '|' {
			reifier := rdflibgo.NewBNode()
			p.g.Add(reifier, reifiesPred, tt)
			if err := p.readAnnotationBlock(reifier, tt); err != nil {
				return err
			}
			continue
		}

		break
	}
	return nil
}

// readEmptyBNodeOnly reads [] but rejects [pred obj] — used inside reified triples.
func (p *turtleParser) readEmptyBNodeOnly() (rdflibgo.BNode, error) {
	p.pos++ // skip '['
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		return rdflibgo.NewBNode(), nil
	}
	return rdflibgo.BNode{}, p.errorf("blank node property list not allowed in reified triple (only [] is allowed)")
}

// readReifierID reads a reifier identifier: IRI, prefixed name, or blank node.
func (p *turtleParser) readReifierID() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("expected reifier identifier")
	}
	ch := p.input[p.pos]
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readAnnotationBlock reads {| predicateObjectList |} and asserts triples on the reifier.
func (p *turtleParser) readAnnotationBlock(reifier rdflibgo.Subject, _ rdflibgo.TripleTerm) error {
	// Consume "{|"
	p.pos += 2
	p.skipWS()

	// Check for empty annotation block — that's an error
	if p.pos+1 < len(p.input) && p.input[p.pos] == '|' && p.input[p.pos+1] == '}' {
		return p.errorf("empty annotation block not allowed")
	}

	if err := p.predicateObjectList(reifier); err != nil {
		return err
	}

	p.skipWS()
	if p.pos+1 >= len(p.input) || p.input[p.pos] != '|' || p.input[p.pos+1] != '}' {
		return p.errorf("expected '|}' to close annotation block")
	}
	p.pos += 2
	return nil
}

// readSubject parses a subject: IRI, prefixed name, blank node, or collection.
func (p *turtleParser) readSubject() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input, expected subject")
	}
	ch := p.input[p.pos]

	// Reified triple as subject: << s p o >> or << s p o ~ id >>
	if ch == '<' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
		return p.readReifiedTriple()
	}
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	if ch == '[' {
		return p.readBlankNodePropertyList()
	}
	if ch == '(' {
		term, err := p.readCollection()
		if err != nil {
			return nil, err
		}
		if subj, ok := term.(rdflibgo.Subject); ok {
			return subj, nil
		}
		return nil, p.errorf("collection as subject must be a node")
	}

	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readVerb parses a predicate: 'a' | IRI | prefixed name.
func (p *turtleParser) readVerb() (rdflibgo.URIRef, error) {
	p.skipWS()
	// Check for 'a' keyword
	if p.pos < len(p.input) && p.input[p.pos] == 'a' {
		// Make sure it's not part of a longer name
		next := p.pos + 1
		if next >= len(p.input) || isDelimiter(p.input[next]) {
			p.pos++
			return rdflibgo.RDF.Type, nil
		}
	}

	return p.readPredicate()
}

func (p *turtleParser) readPredicate() (rdflibgo.URIRef, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return rdflibgo.URIRef{}, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	uri, err := p.readPrefixedName()
	if err != nil {
		return rdflibgo.URIRef{}, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readObject parses an object term.
func (p *turtleParser) readObject() (rdflibgo.Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input")
	}
	ch := p.input[p.pos]

	// Triple term <<( s p o )>> or reified triple << s p o >>
	if ch == '<' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
		return p.readTripleTermOrReified()
	}
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	if ch == '[' {
		return p.readBlankNodePropertyList()
	}
	if ch == '(' {
		return p.readCollection()
	}
	if ch == '"' || ch == '\'' {
		return p.readLiteral()
	}

	// Try numeric literal
	if ch == '+' || ch == '-' || (ch >= '0' && ch <= '9') || ch == '.' {
		if lit, ok := p.tryNumeric(); ok {
			return lit, nil
		}
	}

	// Boolean keywords
	if p.startsWith("true") && (p.pos+4 >= len(p.input) || isDelimiter(p.input[p.pos+4])) {
		p.pos += 4
		return rdflibgo.NewLiteral(true), nil
	}
	if p.startsWith("false") && (p.pos+5 >= len(p.input) || isDelimiter(p.input[p.pos+5])) {
		p.pos += 5
		return rdflibgo.NewLiteral(false), nil
	}

	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readIRI reads <...> and returns the IRI string (without angle brackets).
func (p *turtleParser) readIRI() (string, error) {
	if !p.expect('<') {
		return "", p.errorf("expected '<'")
	}
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '>' {
			iri := p.input[start:p.pos]
			p.pos++
			unescaped, err := p.unescapeIRI(iri)
			if err != nil {
				return "", err
			}
			if err := validateIRI(unescaped); err != nil {
				return "", p.errorf("%s", err)
			}
			return unescaped, nil
		}
		if ch == '\\' {
			p.pos += 2 // skip escape (handled in unescape)
			continue
		}
		// Reject characters not allowed in IRIs per Turtle grammar.
		if ch <= 0x20 || ch == '{' || ch == '}' || ch == '|' || ch == '^' || ch == '`' {
			return "", p.errorf("invalid character %q in IRI", ch)
		}
		p.pos++
	}
	return "", p.errorf("unterminated IRI")
}

// readPrefixedName reads prefix:local and returns the full URI.
func (p *turtleParser) readPrefixedName() (string, error) {
	prefix := p.readPrefixName()
	if !p.expect(':') {
		return "", p.errorf("expected ':' in prefixed name")
	}
	local, err := p.readLocalName()
	if err != nil {
		return "", err
	}
	ns, ok := p.prefixes[prefix]
	if !ok {
		return "", p.errorf("undefined prefix %q", prefix)
	}
	return ns + unescapeLocalName(local), nil
}

// unescapeLocalName processes backslash escapes and percent-encoding in local names.
// In Turtle, local names allow \-escaped reserved characters like \#, \~, \., etc.
// The backslash is removed, yielding the literal character.
func unescapeLocalName(s string) string {
	if !strings.ContainsAny(s, "\\") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			sb.WriteByte(s[i])
		} else {
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// readBlankNodeLabel reads _:label per the BLANK_NODE_LABEL grammar rule.
func (p *turtleParser) readBlankNodeLabel() (rdflibgo.BNode, error) {
	p.pos += 2 // skip "_:"
	start := p.pos
	if p.pos >= len(p.input) {
		return rdflibgo.BNode{}, p.errorf("empty blank node label after _:")
	}
	// First char: PN_CHARS_U | [0-9]
	r, size := utf8.DecodeRuneInString(p.input[p.pos:])
	if !isPNCharsU(r) && !(r >= '0' && r <= '9') {
		return rdflibgo.BNode{}, p.errorf("invalid blank node label start: %c", r)
	}
	p.pos += size
	// Subsequent chars: (PN_CHARS | '.')*
	for p.pos < len(p.input) {
		r, size = utf8.DecodeRuneInString(p.input[p.pos:])
		if isPNChar(r) || r == '.' {
			p.pos += size
		} else {
			break
		}
	}
	// Trim trailing dots.
	for p.pos > start && p.input[p.pos-1] == '.' {
		p.pos--
	}
	label := p.input[start:p.pos]
	if label == "" {
		return rdflibgo.BNode{}, p.errorf("empty blank node label after _:")
	}
	return rdflibgo.NewBNode(label), nil
}

// readBlankNodePropertyList reads [...].
func (p *turtleParser) readBlankNodePropertyList() (rdflibgo.BNode, error) {
	p.pos++ // skip '['
	p.skipWS()

	b := rdflibgo.NewBNode()

	// Empty blank node []
	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		return b, nil
	}

	if err := p.predicateObjectList(b); err != nil {
		return rdflibgo.BNode{}, err
	}
	p.skipWS()
	if !p.expect(']') {
		return rdflibgo.BNode{}, p.errorf("expected ']'")
	}
	return b, nil
}

// readCollection reads (...) and builds rdf:List triples.
func (p *turtleParser) readCollection() (rdflibgo.Term, error) {
	p.pos++ // skip '('
	p.skipWS()

	// Empty collection
	if p.pos < len(p.input) && p.input[p.pos] == ')' {
		p.pos++
		return rdflibgo.RDF.Nil, nil
	}

	var items []rdflibgo.Term
	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			return nil, p.errorf("unterminated collection")
		}
		if p.input[p.pos] == ')' {
			p.pos++
			break
		}
		obj, err := p.readObject()
		if err != nil {
			return nil, err
		}
		items = append(items, obj)
	}

	// Build rdf:List chain
	if len(items) == 0 {
		return rdflibgo.RDF.Nil, nil
	}

	head := rdflibgo.NewBNode()
	current := head
	for i, item := range items {
		p.g.Add(current, rdflibgo.RDF.First, item)
		if i < len(items)-1 {
			next := rdflibgo.NewBNode()
			p.g.Add(current, rdflibgo.RDF.Rest, next)
			current = next
		} else {
			p.g.Add(current, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
		}
	}
	return head, nil
}

// readLiteral reads a string literal with optional language tag or datatype.
func (p *turtleParser) readLiteral() (rdflibgo.Literal, error) {
	quote := p.input[p.pos]
	p.pos++

	// Check for triple-quoted string
	longString := false
	if p.pos+1 < len(p.input) && p.input[p.pos] == quote && p.input[p.pos+1] == quote {
		p.pos += 2
		longString = true
	}

	var sb strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]

		if ch == '\\' {
			p.pos++
			if p.pos >= len(p.input) {
				return rdflibgo.Literal{}, p.errorf("unterminated escape")
			}
			escaped, err := p.readEscape()
			if err != nil {
				return rdflibgo.Literal{}, err
			}
			sb.WriteString(escaped)
			continue
		}

		if longString {
			if ch == quote && p.pos+2 < len(p.input) && p.input[p.pos+1] == quote && p.input[p.pos+2] == quote {
				p.pos += 3
				goto done
			}
			r, size := utf8.DecodeRuneInString(p.input[p.pos:])
			sb.WriteRune(r)
			if ch == '\n' {
				p.line++
				p.col = 1
			}
			p.pos += size
		} else {
			if ch == quote {
				p.pos++
				goto done
			}
			if ch == '\n' || ch == '\r' {
				return rdflibgo.Literal{}, p.errorf("newline in short string")
			}
			r, size := utf8.DecodeRuneInString(p.input[p.pos:])
			sb.WriteRune(r)
			p.pos += size
		}
	}
	return rdflibgo.Literal{}, p.errorf("unterminated string literal")

done:
	value := sb.String()
	var lopts []rdflibgo.LiteralOption

	// Language tag (with optional direction) or datatype
	if p.pos < len(p.input) && p.input[p.pos] == '@' {
		p.pos++
		lang, err := p.readLangTag()
		if err != nil {
			return rdflibgo.Literal{}, err
		}
		// Check for directional language tag: lang--dir
		if idx := strings.Index(lang, "--"); idx >= 0 {
			dir := lang[idx+2:]
			lang = lang[:idx]
			if dir != "ltr" && dir != "rtl" {
				return rdflibgo.Literal{}, p.errorf("invalid base direction %q (must be ltr or rtl)", dir)
			}
			lopts = append(lopts, rdflibgo.WithLang(lang), rdflibgo.WithDir(dir))
		} else {
			lopts = append(lopts, rdflibgo.WithLang(lang))
		}
	} else if p.pos+1 < len(p.input) && p.input[p.pos] == '^' && p.input[p.pos+1] == '^' {
		p.pos += 2
		dt, err := p.readDatatypeIRI()
		if err != nil {
			return rdflibgo.Literal{}, err
		}
		lopts = append(lopts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(dt)))
	}

	return rdflibgo.NewLiteral(value, lopts...), nil
}

// readEscape handles escape sequences.
func (p *turtleParser) readEscape() (string, error) {
	ch := p.input[p.pos]
	p.pos++
	switch ch {
	case 'n':
		return "\n", nil
	case 'r':
		return "\r", nil
	case 't':
		return "\t", nil
	case 'b':
		return "\b", nil
	case 'f':
		return "\f", nil
	case '\\':
		return "\\", nil
	case '"':
		return "\"", nil
	case '\'':
		return "'", nil
	case 'u':
		return p.readUnicodeEscape(4)
	case 'U':
		return p.readUnicodeEscape(8)
	default:
		return "", p.errorf("unknown escape \\%c", ch)
	}
}

func (p *turtleParser) readUnicodeEscape(n int) (string, error) {
	if p.pos+n > len(p.input) {
		return "", p.errorf("truncated unicode escape")
	}
	hex := p.input[p.pos : p.pos+n]
	p.pos += n
	code, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return "", p.errorf("invalid unicode escape: %s", hex)
	}
	// Reject surrogate code points (U+D800..U+DFFF).
	if code >= 0xD800 && code <= 0xDFFF {
		return "", p.errorf("invalid surrogate in unicode escape: %s", hex)
	}
	return string(rune(code)), nil
}

// tryNumeric attempts to parse a numeric literal.
func (p *turtleParser) tryNumeric() (rdflibgo.Literal, bool) {
	start := p.pos

	// Optional sign
	if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
		p.pos++
	}

	hasDigitsBefore := false
	for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		hasDigitsBefore = true
		p.pos++
	}

	hasDot := false
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		next := byte(0)
		if p.pos+1 < len(p.input) {
			next = p.input[p.pos+1]
		}
		if next >= '0' && next <= '9' {
			hasDot = true
			p.pos++ // skip '.'
			for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
				p.pos++
			}
		} else if hasDigitsBefore && (next == 'e' || next == 'E') {
			// e.g. 123.E+1 — dot followed by exponent, no fractional digits
			hasDot = true
			p.pos++ // skip '.'
		} else if !hasDigitsBefore {
			p.pos = start
			return rdflibgo.Literal{}, false
		}
	}

	hasExp := false
	if p.pos < len(p.input) && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		hasExp = true
		p.pos++
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			p.pos++
		}
		if p.pos >= len(p.input) || p.input[p.pos] < '0' || p.input[p.pos] > '9' {
			p.pos = start
			return rdflibgo.Literal{}, false
		}
		for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}
	}

	if !hasDigitsBefore && !hasDot {
		p.pos = start
		return rdflibgo.Literal{}, false
	}

	lexical := p.input[start:p.pos]

	var dt rdflibgo.URIRef
	switch {
	case hasExp:
		dt = rdflibgo.XSDDouble
	case hasDot:
		dt = rdflibgo.XSDDecimal
	default:
		dt = rdflibgo.XSDInteger
	}

	return rdflibgo.NewLiteral(lexical, rdflibgo.WithDatatype(dt)), true
}

func (p *turtleParser) readLangTag() (string, error) {
	start := p.pos
	// First char must be a letter.
	if p.pos >= len(p.input) || !((p.input[p.pos] >= 'a' && p.input[p.pos] <= 'z') || (p.input[p.pos] >= 'A' && p.input[p.pos] <= 'Z')) {
		return "", p.errorf("invalid language tag: must start with a letter")
	}
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '-' || (ch >= '0' && ch <= '9') {
			p.pos++
		} else {
			break
		}
	}
	return p.input[start:p.pos], nil
}

// readTripleTermOrReified reads either <<( s p o )>> (triple term) or << s p o >> (reified triple in object position).
func (p *turtleParser) readTripleTermOrReified() (rdflibgo.Term, error) {
	p.pos += 2 // skip "<<"
	p.skipWS()

	// Triple term: <<( s p o )>>
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		return p.readTripleTermInner()
	}

	// Reified triple: << s p o >> or << s p o ~ id >>
	return p.readReifiedTripleInner()
}

// readTripleTermInner parses the inner part of <<( s p o )>> after "<<" has been consumed.
func (p *turtleParser) readTripleTermInner() (rdflibgo.TripleTerm, error) {
	p.pos++ // skip '('
	p.skipWS()

	subj, err := p.readTripleTermSubject()
	if err != nil {
		return rdflibgo.TripleTerm{}, err
	}

	pred, err := p.readPredicate()
	if err != nil {
		return rdflibgo.TripleTerm{}, err
	}

	obj, err := p.readObject()
	if err != nil {
		return rdflibgo.TripleTerm{}, err
	}

	p.skipWS()
	if !p.expect(')') {
		return rdflibgo.TripleTerm{}, p.errorf("expected ')' in triple term")
	}
	p.skipWS()
	if !p.startsWith(">>") {
		return rdflibgo.TripleTerm{}, p.errorf("expected '>>' to close triple term")
	}
	p.pos += 2

	return rdflibgo.NewTripleTerm(subj, pred, obj), nil
}

// readTripleTermSubject reads a subject for a triple term (IRI or blank node, not a reified triple).
func (p *turtleParser) readTripleTermSubject() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input, expected triple term subject")
	}
	ch := p.input[p.pos]
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readReifiedTriple reads << s p o >> or << s p o ~ id >> as a subject.
// The reified triple creates a node (bnode or named) that gets rdf:reifies <<(s p o)>>.
func (p *turtleParser) readReifiedTriple() (rdflibgo.Subject, error) {
	p.pos += 2 // skip "<<"
	p.skipWS()

	// Check for "(" — that would be a triple term, which is not valid as subject
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		return nil, p.errorf("triple term <<( ... )>> cannot be used as subject")
	}

	return p.readReifiedTripleInner()
}

// readReifiedTripleInner parses the inside of << s p o [~ id] >> after "<<" has been consumed.
// Returns the reifier node (bnode or IRI).
func (p *turtleParser) readReifiedTripleInner() (rdflibgo.Subject, error) {
	// Read inner subject — cannot be a literal or collection or blank node property list
	subj, err := p.readReifiedInnerSubject()
	if err != nil {
		return nil, err
	}

	pred, err := p.readPredicate()
	if err != nil {
		return nil, err
	}

	obj, err := p.readReifiedInnerObject()
	if err != nil {
		return nil, err
	}

	p.skipWS()

	// Optional reifier: ~ id
	var reifier rdflibgo.Subject
	if p.pos < len(p.input) && p.input[p.pos] == '~' {
		p.pos++ // skip ~
		p.skipWS()
		// Check if there's an identifier or just >>
		if p.pos < len(p.input) && p.input[p.pos] != '>' {
			reifier, err = p.readReifierID()
			if err != nil {
				return nil, err
			}
		} else {
			reifier = rdflibgo.NewBNode()
		}
	} else {
		reifier = rdflibgo.NewBNode()
	}

	p.skipWS()
	if !p.startsWith(">>") {
		return nil, p.errorf("expected '>>' to close reified triple")
	}
	p.pos += 2

	// Emit the rdf:reifies triple
	tt := rdflibgo.NewTripleTerm(subj, pred, obj)
	p.g.Add(reifier, rdflibgo.RDFReifies, tt)

	return reifier, nil
}

// readReifiedInnerSubject reads the subject inside a reified triple.
// IRI, prefixed name, blank node label, empty [], or nested reified triple.
func (p *turtleParser) readReifiedInnerSubject() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input in reified triple subject")
	}
	ch := p.input[p.pos]
	// Nested reified triple
	if ch == '<' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
		return p.readReifiedTriple()
	}
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	if ch == '[' {
		return p.readBlankNodePropertyList()
	}
	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readReifiedInnerObject reads the object inside a reified triple.
// IRI, prefixed name, blank node, literal, or nested reified triple.
// Collections and blank node property lists are NOT allowed.
func (p *turtleParser) readReifiedInnerObject() (rdflibgo.Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input in reified triple object")
	}
	ch := p.input[p.pos]

	// Nested reified triple or triple term
	if ch == '<' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
		return p.readTripleTermOrReified()
	}
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	if ch == '"' || ch == '\'' {
		return p.readLiteral()
	}

	// Try numeric literal
	if ch == '+' || ch == '-' || (ch >= '0' && ch <= '9') || ch == '.' {
		if lit, ok := p.tryNumeric(); ok {
			return lit, nil
		}
	}

	// Boolean keywords
	if p.startsWith("true") && (p.pos+4 >= len(p.input) || isDelimiter(p.input[p.pos+4])) {
		p.pos += 4
		return rdflibgo.NewLiteral(true), nil
	}
	if p.startsWith("false") && (p.pos+5 >= len(p.input) || isDelimiter(p.input[p.pos+5])) {
		p.pos += 5
		return rdflibgo.NewLiteral(false), nil
	}

	// Collection not allowed in reified triple
	if ch == '(' {
		return nil, p.errorf("collection not allowed in reified triple")
	}
	// Only empty blank node [] allowed in reified triple; [pred obj] is not.
	if ch == '[' {
		return p.readEmptyBNodeOnly()
	}

	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

func (p *turtleParser) readDatatypeIRI() (string, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return "", err
		}
		return p.resolveIRI(iri), nil
	}
	// Prefixed name
	return p.readPrefixedName()
}

// --- Helper methods ---

func (p *turtleParser) readPrefixName() string {
	start := p.pos
	for p.pos < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r == ':' || (r < 128 && isDelimiter(byte(r))) {
			break
		}
		if p.pos == start {
			// First char: must be PN_CHARS_BASE
			if !isPNCharsBase(r) {
				break
			}
		} else {
			// Subsequent chars: PN_CHARS | '.'
			if !isPNChar(r) && r != '.' {
				break
			}
		}
		p.pos += size
	}
	// Trim trailing dots (not allowed at end of prefix name).
	for p.pos > start && p.input[p.pos-1] == '.' {
		p.pos--
	}
	return p.input[start:p.pos]
}

func (p *turtleParser) readLocalName() (string, error) {
	start := p.pos
	first := true
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\\' && p.pos+1 < len(p.input) {
			next := p.input[p.pos+1]
			// Only reserved character escapes allowed in local names, not \u or \U.
			if next == 'u' || next == 'U' {
				return "", p.errorf("\\%c escape not allowed in local name", next)
			}
			p.pos += 2
			first = false
			continue
		}
		if ch == '%' {
			// Validate percent encoding: must be %HH.
			if p.pos+2 >= len(p.input) || !isHexDigit(p.input[p.pos+1]) || !isHexDigit(p.input[p.pos+2]) {
				return "", p.errorf("invalid percent encoding in local name")
			}
			p.pos += 3
			first = false
			continue
		}
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if first {
			// First char of local name: PN_CHARS_U | ':' | [0-9] | PLX (handled above)
			if !isPNCharsU(r) && r != ':' && !(r >= '0' && r <= '9') {
				break
			}
		} else {
			// Subsequent: PN_CHARS | '.' | ':' | PLX (handled above)
			if r == ':' || r == '.' {
				p.pos += size
				continue
			}
			if r == ';' || r == ',' || r == '[' || r == ']' || r == '(' || r == ')' || r == '#' {
				break
			}
			if r < 128 && isDelimiter(byte(r)) {
				break
			}
			if !isPNChar(r) {
				break
			}
		}
		p.pos += size
		first = false
	}
	// Trim trailing dots (not allowed at end of local name).
	for p.pos > start && p.input[p.pos-1] == '.' {
		p.pos--
	}
	return p.input[start:p.pos], nil
}

func (p *turtleParser) skipWS() {
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' {
			p.pos++
			p.col++
		} else if ch == '\n' {
			p.pos++
			p.line++
			p.col = 1
		} else if ch == '#' {
			// Comment: skip to end of line
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

func (p *turtleParser) expect(ch byte) bool {
	if p.pos < len(p.input) && p.input[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}

func (p *turtleParser) startsWith(s string) bool {
	return strings.HasPrefix(p.input[p.pos:], s)
}

func (p *turtleParser) matchKeywordCI(kw string) bool {
	if p.pos+len(kw) > len(p.input) {
		return false
	}
	candidate := p.input[p.pos : p.pos+len(kw)]
	if !strings.EqualFold(candidate, kw) {
		return false
	}
	// Must be followed by whitespace or EOF
	after := p.pos + len(kw)
	if after < len(p.input) && !isWhitespace(p.input[after]) {
		return false
	}
	return true
}

func (p *turtleParser) resolveIRI(iri string) string {
	if p.base == "" || isAbsoluteIRI(iri) {
		return iri
	}
	b, err := url.Parse(p.base)
	if err != nil {
		return iri
	}
	ref, err := url.Parse(iri)
	if err != nil {
		return iri
	}
	resolved := b.ResolveReference(ref).String()
	// Go's url.ResolveReference drops an empty fragment. In RDF, <#> resolved
	// against a base must preserve the '#' separator.
	if strings.Contains(iri, "#") && !strings.Contains(resolved, "#") {
		resolved += "#"
	}
	return resolved
}

func (p *turtleParser) unescapeIRI(s string) (string, error) {
	if !strings.ContainsRune(s, '\\') {
		return s, nil
	}
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'u':
				if i+5 > len(s) {
					return "", p.errorf("truncated \\u escape in IRI")
				}
				code, err := strconv.ParseUint(s[i+1:i+5], 16, 32)
				if err != nil {
					return "", p.errorf("invalid \\u escape in IRI: %s", s[i+1:i+5])
				}
				if code >= 0xD800 && code <= 0xDFFF {
					return "", p.errorf("invalid surrogate in IRI escape: %s", s[i+1:i+5])
				}
				sb.WriteRune(rune(code))
				i += 5
			case 'U':
				if i+9 > len(s) {
					return "", p.errorf("truncated \\U escape in IRI")
				}
				code, err := strconv.ParseUint(s[i+1:i+9], 16, 32)
				if err != nil {
					return "", p.errorf("invalid \\U escape in IRI: %s", s[i+1:i+9])
				}
				if code >= 0xD800 && code <= 0xDFFF {
					return "", p.errorf("invalid surrogate in IRI escape: %s", s[i+1:i+9])
				}
				sb.WriteRune(rune(code))
				i += 9
			default:
				return "", p.errorf("unknown escape \\%c in IRI", s[i])
			}
		} else {
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String(), nil
}

func (p *turtleParser) errorf(format string, args ...any) error {
	return fmt.Errorf("turtle parse error at line %d: "+format, append([]any{p.line}, args...)...)
}

func isDelimiter(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '<' || ch == '>' || ch == '"' || ch == '\'' || ch == '{' || ch == '}' || ch == '|' || ch == '^' || ch == '`' || ch == ')' || ch == '~'
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

// isPNChar matches PN_CHARS (PN_CHARS_U | '-' | digit | combining marks).
func isPNChar(r rune) bool {
	return isPNCharsU(r) ||
		r == '-' ||
		(r >= '0' && r <= '9') ||
		r == 0x00B7 ||
		(r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x203F && r <= 0x2040)
}

func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

// validateIRI checks that the unescaped IRI doesn't contain characters
// forbidden by the Turtle grammar (space, <, >, {, }, |, ^, `, controls).
func validateIRI(s string) error {
	for _, r := range s {
		if r <= 0x20 || r == '<' || r == '>' || r == '{' || r == '}' || r == '|' || r == '^' || r == '`' {
			return fmt.Errorf("invalid character U+%04X in IRI", r)
		}
	}
	return nil
}

func isAbsoluteIRI(s string) bool {
	// Has scheme: starts with letter followed by letters/digits/+/-./:
	colon := strings.Index(s, ":")
	if colon <= 0 {
		return false
	}
	for i := 0; i < colon; i++ {
		ch := s[i]
		if i == 0 {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
				return false
			}
		} else {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '+' || ch == '-' || ch == '.') {
				return false
			}
		}
	}
	return true
}
