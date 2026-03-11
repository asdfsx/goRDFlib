package reasoning

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

func BenchmarkOWLRL_SymmetricProperty(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				g := graph.NewGraph()
				p := ex.Term("sym")
				g.Add(p, namespace.RDF.Type, namespace.OWL.SymmetricProperty)
				for j := 0; j < n; j++ {
					s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", j))
					o := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/o%d", j))
					g.Add(s, p, o)
				}
				b.StartTimer()
				OWLRLClosure(g)
			}
		})
	}
}

func BenchmarkOWLRL_TransitiveChain(b *testing.B) {
	for _, depth := range []int{10, 50, 200} {
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				g := graph.NewGraph()
				p := ex.Term("trans")
				g.Add(p, namespace.RDF.Type, namespace.OWL.TransitiveProperty)
				for j := 0; j < depth; j++ {
					s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", j))
					o := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", j+1))
					g.Add(s, p, o)
				}
				b.StartTimer()
				OWLRLClosure(g)
			}
		})
	}
}

func BenchmarkOWLRL_SameAsCluster(b *testing.B) {
	for _, n := range []int{5, 20, 50} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				g := graph.NewGraph()
				// Create a chain of sameAs
				for j := 0; j < n-1; j++ {
					a := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/i%d", j))
					bTerm := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/i%d", j+1))
					g.Add(a, namespace.OWL.SameAs, bTerm)
				}
				// Add some data triples
				p := ex.Term("p")
				for j := 0; j < n; j++ {
					s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/i%d", j))
					o := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/val%d", j))
					g.Add(s, p, o)
				}
				b.StartTimer()
				OWLRLClosure(g)
			}
		})
	}
}

func BenchmarkOWLRL_Combined(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		g := graph.NewGraph()

		sym := ex.Term("sym")
		trans := ex.Term("trans")
		inv1 := ex.Term("inv1")
		inv2 := ex.Term("inv2")

		g.Add(sym, namespace.RDF.Type, namespace.OWL.SymmetricProperty)
		g.Add(trans, namespace.RDF.Type, namespace.OWL.TransitiveProperty)
		g.Add(inv1, namespace.OWL.InverseOf, inv2)

		for j := 0; j < 100; j++ {
			s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", j))
			o := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/o%d", j))
			g.Add(s, sym, o)
		}
		for j := 0; j < 50; j++ {
			s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/t%d", j))
			o := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/t%d", j+1))
			g.Add(s, trans, o)
		}
		for j := 0; j < 50; j++ {
			s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/a%d", j))
			o := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/b%d", j))
			g.Add(s, inv1, o)
		}

		b.StartTimer()
		OWLRLClosure(g)
	}
}
