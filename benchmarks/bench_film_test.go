package benchmarks_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// ============================================================================
// Dataset sizes inspired by dgraph-benchmarks:
//   - 10k   (~golden data)
//   - 100k  (medium)
//   - 1M    (large — matches dgraph 1million.rdf)
// ============================================================================

// --- Parse N-Triples ---

func BenchmarkParseNTriples_10k(b *testing.B) {
	data := generateFilmNTriples(10_000)
	r := strings.NewReader(data)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Reset(data)
		g := graph.NewGraph()
		nt.Parse(g, r)
	}
}

func BenchmarkParseNTriples_100k(b *testing.B) {
	data := generateFilmNTriples(100_000)
	r := strings.NewReader(data)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Reset(data)
		g := graph.NewGraph()
		nt.Parse(g, r)
	}
}

// --- Parse Turtle ---

func BenchmarkParseTurtle_10k(b *testing.B) {
	data := generateFilmTurtle(10_000)
	r := strings.NewReader(data)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Reset(data)
		g := graph.NewGraph()
		turtle.Parse(g, r)
	}
}

func BenchmarkParseTurtle_100k(b *testing.B) {
	data := generateFilmTurtle(100_000)
	r := strings.NewReader(data)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Reset(data)
		g := graph.NewGraph()
		turtle.Parse(g, r)
	}
}

// --- Serialize N-Triples ---

func BenchmarkSerializeNTriples_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		nt.Serialize(g, &buf)
	}
}

// --- Serialize Turtle ---

func BenchmarkSerializeTurtle_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		turtle.Serialize(g, &buf)
	}
}

// --- Store: bulk add ---

func BenchmarkStoreAdd_100k(b *testing.B) {
	// Measure time to build a 100k-triple graph from scratch.
	data := generateFilmNTriples(100_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g := graph.NewGraph()
		nt.Parse(g, strings.NewReader(data))
	}
}

// --- Store: pattern lookup on large graph ---

func BenchmarkStoreLookup_AllFilms_100k(b *testing.B) {
	g := generateFilmDataset(100_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		for range g.Triples(nil, &rdfType, &fbFilm) {
			count++
		}
	}
}

func BenchmarkStoreLookup_FilmsByDirector_100k(b *testing.B) {
	g := generateFilmDataset(100_000)
	director := term.NewURIRefUnsafe("http://freebase.com/director/0")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		for range g.Triples(nil, &fbDirectedBy, &director) {
			count++
		}
	}
}

// --- SPARQL on film data ---

func BenchmarkSPARQL_FilmsByDirector_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	q := `SELECT ?film ?title WHERE {
		?film <http://freebase.com/film.film.directed_by> <http://freebase.com/director/0> .
		?film <http://www.w3.org/2000/01/rdf-schema#label> ?title .
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

func BenchmarkSPARQL_FilmsByDirector_100k(b *testing.B) {
	g := generateFilmDataset(100_000)
	q := `SELECT ?film ?title WHERE {
		?film <http://freebase.com/film.film.directed_by> <http://freebase.com/director/0> .
		?film <http://www.w3.org/2000/01/rdf-schema#label> ?title .
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

func BenchmarkSPARQL_CountFilmsByGenre_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	q := `SELECT ?genre (COUNT(?film) AS ?count) WHERE {
		?film a <http://freebase.com/film.film> .
		?film <http://freebase.com/film.film.genre> ?genre .
	} GROUP BY ?genre ORDER BY DESC(?count)`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

func BenchmarkSPARQL_CountFilmsByGenre_100k(b *testing.B) {
	g := generateFilmDataset(100_000)
	q := `SELECT ?genre (COUNT(?film) AS ?count) WHERE {
		?film a <http://freebase.com/film.film> .
		?film <http://freebase.com/film.film.genre> ?genre .
	} GROUP BY ?genre ORDER BY DESC(?count)`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

func BenchmarkSPARQL_ActorsInFilm_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	q := `SELECT ?actor ?name WHERE {
		<http://freebase.com/film/0> <http://freebase.com/film.film.starring> ?perf .
		?perf <http://freebase.com/film.performance.actor> ?actor .
		?actor <http://www.w3.org/2000/01/rdf-schema#label> ?name .
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

func BenchmarkSPARQL_Top10Films_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	q := `SELECT ?film ?title ?rating WHERE {
		?film a <http://freebase.com/film.film> .
		?film <http://www.w3.org/2000/01/rdf-schema#label> ?title .
		?film <http://freebase.com/film.film.rating> ?rating .
	} ORDER BY DESC(?rating) LIMIT 10`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

func BenchmarkSPARQL_Top10Films_100k(b *testing.B) {
	g := generateFilmDataset(100_000)
	q := `SELECT ?film ?title ?rating WHERE {
		?film a <http://freebase.com/film.film> .
		?film <http://www.w3.org/2000/01/rdf-schema#label> ?title .
		?film <http://freebase.com/film.film.rating> ?rating .
	} ORDER BY DESC(?rating) LIMIT 10`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

// Multi-hop: find all actors who worked with a specific director
func BenchmarkSPARQL_ActorsByDirector_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	q := `SELECT DISTINCT ?actor ?actorName WHERE {
		?film <http://freebase.com/film.film.directed_by> <http://freebase.com/director/0> .
		?film <http://freebase.com/film.film.starring> ?perf .
		?perf <http://freebase.com/film.performance.actor> ?actor .
		?actor <http://www.w3.org/2000/01/rdf-schema#label> ?actorName .
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

func BenchmarkSPARQL_ActorsByDirector_100k(b *testing.B) {
	g := generateFilmDataset(100_000)
	q := `SELECT DISTINCT ?actor ?actorName WHERE {
		?film <http://freebase.com/film.film.directed_by> <http://freebase.com/director/0> .
		?film <http://freebase.com/film.film.starring> ?perf .
		?perf <http://freebase.com/film.performance.actor> ?actor .
		?actor <http://www.w3.org/2000/01/rdf-schema#label> ?actorName .
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

// OPTIONAL pattern
func BenchmarkSPARQL_FilmsOptionalRating_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	q := `SELECT ?film ?title ?rating WHERE {
		?film a <http://freebase.com/film.film> .
		?film <http://www.w3.org/2000/01/rdf-schema#label> ?title .
		OPTIONAL { ?film <http://freebase.com/film.film.rating> ?rating }
	} LIMIT 100`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

// FILTER with string matching
func BenchmarkSPARQL_FilterRegex_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	q := fmt.Sprintf(`SELECT ?s ?label WHERE {
		?s <http://www.w3.org/2000/01/rdf-schema#label> ?label .
		FILTER(REGEX(?label, "^Film [1-9]"))
	} LIMIT 50`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

// ASK query
func BenchmarkSPARQL_ASK_FilmExists_100k(b *testing.B) {
	g := generateFilmDataset(100_000)
	q := `ASK {
		<http://freebase.com/film/42> a <http://freebase.com/film.film> .
		<http://freebase.com/film/42> <http://freebase.com/film.film.directed_by> ?dir .
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}

// CONSTRUCT
func BenchmarkSPARQL_CONSTRUCT_10k(b *testing.B) {
	g := generateFilmDataset(10_000)
	q := `CONSTRUCT {
		?film <http://www.w3.org/2000/01/rdf-schema#label> ?title .
		?film <http://freebase.com/film.film.rating> ?rating .
	} WHERE {
		?film a <http://freebase.com/film.film> .
		?film <http://www.w3.org/2000/01/rdf-schema#label> ?title .
		?film <http://freebase.com/film.film.rating> ?rating .
	} LIMIT 50`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, q)
	}
}
