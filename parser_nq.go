package rdflibgo

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// NQuadsParser parses N-Quads format RDF.
// Ported from: rdflib.plugins.parsers.nquads.NQuadsParser
type NQuadsParser struct{}

func init() {
	RegisterParser("nquads", func() Parser { return &NQuadsParser{} })
	RegisterParser("nq", func() Parser { return &NQuadsParser{} })
}

func (p *NQuadsParser) Parse(g *Graph, r io.Reader, base string) error {
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if err := parseNQLine(g, line, lineNum); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func parseNQLine(g *Graph, line string, lineNum int) error {
	p := &ntLineParser{line: line, pos: 0, lineNum: lineNum}

	subj, err := p.readNTSubject()
	if err != nil {
		return err
	}
	p.skipSpaces()

	pred, err := p.readNTIRI()
	if err != nil {
		return fmt.Errorf("line %d: predicate: %w", lineNum, err)
	}
	p.skipSpaces()

	obj, err := p.readNTObject()
	if err != nil {
		return err
	}
	p.skipSpaces()

	// Optional 4th element: graph context
	// (ignored for now — added to default graph)
	if p.pos < len(p.line) && p.line[p.pos] != '.' {
		if p.line[p.pos] == '<' {
			// Skip graph IRI
			if _, err := p.readNTIRI(); err != nil {
				return fmt.Errorf("line %d: graph: %w", lineNum, err)
			}
		} else if strings.HasPrefix(p.line[p.pos:], "_:") {
			// Skip graph BNode
			p.readNTBNode()
		}
		p.skipSpaces()
	}

	if !p.expect('.') {
		return fmt.Errorf("line %d: expected '.'", lineNum)
	}

	g.Add(subj, NewURIRefUnsafe(pred), obj)
	return nil
}
