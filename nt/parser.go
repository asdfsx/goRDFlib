package nt

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
)

// Parse parses N-Triples format RDF into the given graph.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		if err := parseNTLine(g, line, lineNum); err != nil {
			if cfg.errorHandler == nil {
				return err
			}
			fixedLine, retry := cfg.errorHandler(lineNum, line, err)
			if retry {
				if err2 := parseNTLine(g, fixedLine, lineNum); err2 != nil {
					return fmt.Errorf("line %d: retry failed: %w", lineNum, err2)
				}
			}
		}
	}
	return scanner.Err()
}

func parseNTLine(g *rdflibgo.Graph, line string, lineNum int) error {
	p := &ntsyntax.LineParser{Line: line, Pos: 0, LineNum: lineNum}

	subj, err := p.ReadSubject()
	if err != nil {
		return err
	}
	p.SkipSpaces()

	pred, err := p.ReadPredicate()
	if err != nil {
		return err
	}
	p.SkipSpaces()

	obj, err := p.ReadObject()
	if err != nil {
		return err
	}
	p.SkipSpaces()

	if !p.Expect('.') {
		return fmt.Errorf("line %d: expected '.'", lineNum)
	}

	g.Add(subj, pred, obj)
	return nil
}
