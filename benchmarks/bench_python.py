"""Benchmarks for Python rdflib — comparable to Go rdflibgo benchmarks."""

import time
import sys

from rdflib import Graph, URIRef, Literal, BNode, Namespace
from rdflib.namespace import XSD, RDF


def bench(name, fn, n=None):
    """Run fn repeatedly, auto-calibrate iterations to get ~1s total."""
    if n is None:
        # warm up & calibrate
        n = 1
        while True:
            t0 = time.perf_counter_ns()
            for _ in range(n):
                fn()
            elapsed = time.perf_counter_ns() - t0
            if elapsed > 500_000_000:  # 0.5s
                break
            n *= 2

    t0 = time.perf_counter_ns()
    for _ in range(n):
        fn()
    elapsed = time.perf_counter_ns() - t0
    ns_per_op = elapsed // n
    print(f"Benchmark{name}\t{n}\t{ns_per_op} ns/op")


# --- Term creation ---

def bench_new_uriref():
    URIRef("http://example.org/resource")

def bench_new_bnode():
    BNode()

def bench_new_literal_string():
    Literal("hello world")

def bench_new_literal_int():
    Literal(42)

# --- N3 serialization ---

_uri = URIRef("http://example.org/resource")
def bench_uriref_n3():
    _uri.n3()

_lit = Literal("hello world")
def bench_literal_n3():
    _lit.n3()

# --- Literal equality ---

_l1 = Literal("1", datatype=XSD.integer)
_l2 = Literal("01", datatype=XSD.integer)
def bench_literal_eq():
    _l1 == _l2

# --- MemoryStore add ---

def bench_store_add():
    g = Graph()
    pred = URIRef("http://example.org/p")
    for i in range(10000):
        g.add((URIRef(f"http://example.org/s{i}"), pred, Literal(i)))

# --- MemoryStore triples lookup ---

def make_lookup_graph():
    g = Graph()
    sub = URIRef("http://example.org/s")
    pred = URIRef("http://example.org/p")
    for i in range(1000):
        g.add((sub, pred, Literal(i)))
    return g, sub, pred

_lg, _ls, _lp = make_lookup_graph()
def bench_store_triples():
    for _ in _lg.triples((_ls, _lp, None)):
        pass

# --- Graph parse (Turtle, small) ---

_turtle_data = """
@prefix ex: <http://example.org/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:Alice a ex:Person ;
    rdfs:label "Alice" ;
    ex:knows ex:Bob .

ex:Bob a ex:Person ;
    rdfs:label "Bob" .
"""

def bench_parse_turtle():
    g = Graph()
    g.parse(data=_turtle_data, format="turtle")

# --- Graph serialize (Turtle, small) ---

_sg = Graph()
_sg.parse(data=_turtle_data, format="turtle")
def bench_serialize_turtle():
    _sg.serialize(format="turtle")

# --- SPARQL query ---

_qg = Graph()
for i in range(100):
    _qg.add((URIRef(f"http://example.org/s{i}"), RDF.type, URIRef("http://example.org/Thing")))
    _qg.add((URIRef(f"http://example.org/s{i}"), URIRef("http://example.org/value"), Literal(i)))

def bench_sparql_select():
    list(_qg.query("SELECT ?s ?v WHERE { ?s a <http://example.org/Thing> ; <http://example.org/value> ?v } LIMIT 50"))


# --- SHACL validation ---

_shacl_shapes_small = """
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
        sh:maxCount 1 ;
        sh:datatype xsd:string ;
    ] ;
    sh:property [
        sh:path ex:age ;
        sh:datatype xsd:integer ;
        sh:minInclusive 0 ;
        sh:maxInclusive 150 ;
    ] .
"""

def _make_shacl_data(n):
    lines = ["@prefix ex: <http://example.org/> .", "@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .", ""]
    for i in range(n):
        lines.append(f'ex:person{i} a ex:Person ; ex:name "Person {i}" ; ex:age {20+i} .')
    return "\n".join(lines)

_shacl_shapes_complex = """
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
        sh:datatype xsd:string ;
        sh:minLength 1 ;
        sh:maxLength 100 ;
    ] ;
    sh:property [
        sh:path ex:email ;
        sh:pattern "^[^@]+@[^@]+$" ;
    ] ;
    sh:property [
        sh:path ex:knows ;
        sh:class ex:Person ;
    ] ;
    sh:property [
        sh:path ex:status ;
        sh:in ( "active" "inactive" "pending" ) ;
    ] .
"""

def _make_shacl_data_complex():
    lines = ["@prefix ex: <http://example.org/> .", ""]
    for i in range(50):
        lines.append(f'ex:p{i} a ex:Person ; ex:name "Person {i}" ; ex:email "p{i}@example.org" ; ex:status "active" .')
        if i > 0:
            lines.append(f'ex:p{i} ex:knows ex:p{i-1} .')
    return "\n".join(lines)

try:
    from pyshacl import validate as shacl_validate

    _shacl_sg_small = Graph().parse(data=_shacl_shapes_small, format="turtle")
    _shacl_dg_small = Graph().parse(data=_make_shacl_data(10), format="turtle")

    _shacl_sg_complex = Graph().parse(data=_shacl_shapes_complex, format="turtle")
    _shacl_dg_complex = Graph().parse(data=_make_shacl_data_complex(), format="turtle")

    _shacl_dg_medium = Graph().parse(data=_make_shacl_data(100), format="turtle")

    def bench_shacl_small():
        shacl_validate(_shacl_dg_small, shacl_graph=_shacl_sg_small)

    def bench_shacl_medium():
        shacl_validate(_shacl_dg_medium, shacl_graph=_shacl_sg_small)

    def bench_shacl_complex():
        shacl_validate(_shacl_dg_complex, shacl_graph=_shacl_sg_complex)

    _has_shacl = True
except ImportError:
    _has_shacl = False


if __name__ == "__main__":
    print(f"Python {sys.version}")
    print(f"rdflib {__import__('rdflib').__version__}")
    if _has_shacl:
        print(f"pyshacl {__import__('pyshacl').__version__}")
    print()

    bench("NewURIRef", bench_new_uriref)
    bench("NewBNode", bench_new_bnode)
    bench("NewLiteralString", bench_new_literal_string)
    bench("NewLiteralInt", bench_new_literal_int)
    bench("URIRefN3", bench_uriref_n3)
    bench("LiteralN3", bench_literal_n3)
    bench("LiteralEq", bench_literal_eq)
    bench("StoreAdd_10k", bench_store_add, n=10)
    bench("StoreTriples_1k", bench_store_triples)
    bench("ParseTurtle", bench_parse_turtle)
    bench("SerializeTurtle", bench_serialize_turtle)
    bench("SPARQLSelect", bench_sparql_select)

    if _has_shacl:
        print()
        bench("SHACLValidateSmall", bench_shacl_small, n=10)
        bench("SHACLValidateMedium", bench_shacl_medium, n=5)
        bench("SHACLValidateComplex", bench_shacl_complex, n=5)
