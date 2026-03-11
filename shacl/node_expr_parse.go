package shacl

import (
	"strconv"
)

// parseNodeExpr parses a node expression from an RDF graph node.
// The expression node can be:
//   - An IRI or literal → ConstantExpr
//   - An RDF list → ListExpr (evaluates each member)
//   - A blank node with shnex:* properties → specific expression type
func parseNodeExpr(g *Graph, node Term) NodeExpr {
	// rdf:nil → empty list expression
	if node.IsIRI() && node.Value() == RDFNil {
		return &ListExpr{IsNil: true}
	}

	// Literal → constant
	if node.IsLiteral() {
		return &ConstantExpr{Value: node}
	}

	// RDF list check (non-blank node that is an IRI with no shnex properties is a constant)
	firstPred := IRI(RDFFirst)
	if g.Has(&node, &firstPred, nil) {
		items := g.RDFList(node)
		members := make([]NodeExpr, len(items))
		for i, item := range items {
			members[i] = parseNodeExpr(g, item)
		}
		return &ListExpr{Members: members}
	}

	// IRI (not rdf:nil, not a list head) → constant
	if node.IsIRI() {
		return &ConstantExpr{Value: node}
	}

	// Blank node — check for shnex:* properties
	return parseBlankNodeExpr(g, node)
}

// parseBlankNodeExpr parses a blank node as a node expression.
func parseBlankNodeExpr(g *Graph, node Term) NodeExpr {
	// Check each shnex:* property to determine expression type

	// shnex:var
	if vals := g.Objects(node, IRI(SHNEX+"var")); len(vals) > 0 {
		return &VarExpr{Name: vals[0].Value()}
	}

	// shnex:pathValues
	if vals := g.Objects(node, IRI(SHNEX+"pathValues")); len(vals) > 0 {
		path := parsePath(g, vals[0])
		var focusNodeExpr NodeExpr
		if fn := g.Objects(node, IRI(SHNEX+"focusNode")); len(fn) > 0 {
			focusNodeExpr = parseNodeExpr(g, fn[0])
		}
		return &PathValuesExpr{Path: path, FocusNode: focusNodeExpr}
	}

	// shnex:count
	if vals := g.Objects(node, IRI(SHNEX+"count")); len(vals) > 0 {
		return &CountExpr{Nodes: parseNodeExpr(g, vals[0])}
	}

	// shnex:sum
	if vals := g.Objects(node, IRI(SHNEX+"sum")); len(vals) > 0 {
		nodesExpr := parseNodeExpr(g, vals[0])
		var flatMapExpr NodeExpr
		if fm := g.Objects(node, IRI(SHNEX+"flatMap")); len(fm) > 0 {
			flatMapExpr = parseNodeExpr(g, fm[0])
		}
		// Check for shnex:nodes which overrides the direct sum argument
		if n := g.Objects(node, IRI(SHNEX+"nodes")); len(n) > 0 {
			nodesExpr2 := parseNodeExpr(g, n[0])
			if flatMapExpr != nil {
				return &SumExpr{Nodes: nodesExpr2, FlatMap: flatMapExpr}
			}
			return &SumExpr{Nodes: nodesExpr2}
		}
		return &SumExpr{Nodes: nodesExpr, FlatMap: flatMapExpr}
	}

	// shnex:min
	if vals := g.Objects(node, IRI(SHNEX+"min")); len(vals) > 0 {
		return &MinExpr{Nodes: parseNodeExpr(g, vals[0])}
	}

	// shnex:max
	if vals := g.Objects(node, IRI(SHNEX+"max")); len(vals) > 0 {
		return &MaxExpr{Nodes: parseNodeExpr(g, vals[0])}
	}

	// shnex:exists
	if vals := g.Objects(node, IRI(SHNEX+"exists")); len(vals) > 0 {
		return &ExistsExpr{Nodes: parseNodeExpr(g, vals[0])}
	}

	// shnex:distinct
	if vals := g.Objects(node, IRI(SHNEX+"distinct")); len(vals) > 0 {
		return &DistinctExpr{Nodes: parseNodeExpr(g, vals[0])}
	}

	// shnex:concat
	if vals := g.Objects(node, IRI(SHNEX+"concat")); len(vals) > 0 {
		return parseConcatExpr(g, vals[0])
	}

	// shnex:intersection
	if vals := g.Objects(node, IRI(SHNEX+"intersection")); len(vals) > 0 {
		return parseIntersectionExpr(g, vals[0])
	}

	// shnex:filterShape
	if vals := g.Objects(node, IRI(SHNEX+"filterShape")); len(vals) > 0 {
		nodes := parseNodesArg(g, node)
		return &FilterShapeExpr{ShapeRef: vals[0], Nodes: nodes}
	}

	// shnex:findFirst
	if vals := g.Objects(node, IRI(SHNEX+"findFirst")); len(vals) > 0 {
		nodes := parseNodesArg(g, node)
		return &FindFirstExpr{ShapeRef: vals[0], Nodes: nodes}
	}

	// shnex:matchAll
	if vals := g.Objects(node, IRI(SHNEX+"matchAll")); len(vals) > 0 {
		nodes := parseNodesArg(g, node)
		return &MatchAllExpr{ShapeRef: vals[0], Nodes: nodes}
	}

	// shnex:nodesMatching
	if vals := g.Objects(node, IRI(SHNEX+"nodesMatching")); len(vals) > 0 {
		return &NodesMatchingExpr{ShapeRef: vals[0]}
	}

	// shnex:instancesOf
	if vals := g.Objects(node, IRI(SHNEX+"instancesOf")); len(vals) > 0 {
		return &InstancesOfExpr{Class: vals[0]}
	}

	// shnex:if
	if vals := g.Objects(node, IRI(SHNEX+"if")); len(vals) > 0 {
		cond := parseNodeExpr(g, vals[0])
		var thenExpr, elseExpr NodeExpr
		if t := g.Objects(node, IRI(SHNEX+"then")); len(t) > 0 {
			thenExpr = parseNodeExpr(g, t[0])
		}
		if e := g.Objects(node, IRI(SHNEX+"else")); len(e) > 0 {
			elseExpr = parseNodeExpr(g, e[0])
		}
		return &IfExpr{Condition: cond, Then: thenExpr, Else: elseExpr}
	}

	// shnex:remove + shnex:nodes
	if vals := g.Objects(node, IRI(SHNEX+"remove")); len(vals) > 0 {
		nodes := parseNodesArg(g, node)
		return &RemoveExpr{Nodes: nodes, ToRemove: parseNodeExpr(g, vals[0])}
	}

	// Expressions that use shnex:nodes as primary with modifiers
	// shnex:flatMap + shnex:nodes
	if fm := g.Objects(node, IRI(SHNEX+"flatMap")); len(fm) > 0 {
		nodes := parseNodesArg(g, node)
		return &FlatMapExpr{Nodes: nodes, MapExpr: parseNodeExpr(g, fm[0])}
	}

	// shnex:orderBy + shnex:nodes
	if ob := g.Objects(node, IRI(SHNEX+"orderBy")); len(ob) > 0 {
		nodes := parseNodesArg(g, node)
		desc := false
		if d := g.Objects(node, IRI(SHNEX+"desc")); len(d) > 0 {
			desc = d[0].Value() == "true"
		}
		return &OrderByExpr{Nodes: nodes, KeyExpr: parseNodeExpr(g, ob[0]), Desc: desc}
	}

	// shnex:nodes + shnex:limit
	if lim := g.Objects(node, IRI(SHNEX+"limit")); len(lim) > 0 {
		nodes := parseNodesArg(g, node)
		limVal, _ := strconv.Atoi(lim[0].Value())
		return &LimitExpr{Nodes: nodes, Limit: limVal}
	}

	// shnex:nodes + shnex:offset
	if off := g.Objects(node, IRI(SHNEX+"offset")); len(off) > 0 {
		nodes := parseNodesArg(g, node)
		offVal, _ := strconv.Atoi(off[0].Value())
		return &OffsetExpr{Nodes: nodes, Offset: offVal}
	}

	// shnex:nodes alone (without modifiers)
	if n := g.Objects(node, IRI(SHNEX+"nodes")); len(n) > 0 {
		return parseNodeExpr(g, n[0])
	}

	// Blank node with no recognized shnex properties → empty expression
	return &EmptyExpr{}
}

// parseNodesArg parses the shnex:nodes argument, defaulting to an empty expression.
func parseNodesArg(g *Graph, node Term) NodeExpr {
	if n := g.Objects(node, IRI(SHNEX+"nodes")); len(n) > 0 {
		return parseNodeExpr(g, n[0])
	}
	return &EmptyExpr{}
}

// parseConcatExpr parses a shnex:concat expression from an RDF list.
func parseConcatExpr(g *Graph, listNode Term) NodeExpr {
	items := g.RDFList(listNode)
	members := make([]NodeExpr, len(items))
	for i, item := range items {
		members[i] = parseNodeExpr(g, item)
	}
	return &ConcatExpr{Members: members}
}

// parseIntersectionExpr parses a shnex:intersection expression from an RDF list.
func parseIntersectionExpr(g *Graph, listNode Term) NodeExpr {
	items := g.RDFList(listNode)
	members := make([]NodeExpr, len(items))
	for i, item := range items {
		members[i] = parseNodeExpr(g, item)
	}
	return &IntersectionExpr{Members: members}
}
