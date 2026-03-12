package store_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/badgerstore"
	"github.com/tggo/goRDFlib/store/sqlitestore"
	"github.com/tggo/goRDFlib/term"
)

const stressN = 3_000_000

func genQuads(n int) []term.Quad {
	quads := make([]term.Quad, n)
	for i := range n {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		pred := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/p%d", i%200))
		obj := term.NewLiteral(fmt.Sprintf("value_%d", i))
		quads[i] = term.Quad{Triple: term.Triple{Subject: subj, Predicate: pred, Object: obj}}
	}
	return quads
}

func memMB() float64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

func TestStress1M_Memory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)
	t.Logf("Generated %d quads", len(quads))

	s := store.NewMemoryStore()
	memBefore := memMB()

	// Ingest
	start := time.Now()
	s.AddN(quads)
	ingestDur := time.Since(start)
	memAfter := memMB()
	t.Logf("MEMORY Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())
	t.Logf("MEMORY RAM: %.1f MB before → %.1f MB after (delta %.1f MB)", memBefore, memAfter, memAfter-memBefore)

	// Len
	start = time.Now()
	n := s.Len(nil)
	t.Logf("MEMORY Len: %d in %v", n, time.Since(start))

	// Full scan
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	t.Logf("MEMORY Full scan: %d triples in %v", count, time.Since(start))

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("MEMORY Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan (returns ~5000 triples)
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count = 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("MEMORY Predicate scan (p42): %d triples in %v", count, time.Since(start))
}

func TestStress1M_Badger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)

	s, err := badgerstore.New(badgerstore.WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	memBefore := memMB()

	// Ingest in batches of 50K (Badger WriteBatch has limits)
	start := time.Now()
	batch := 50_000
	for i := 0; i < len(quads); i += batch {
		end := i + batch
		if end > len(quads) {
			end = len(quads)
		}
		s.AddN(quads[i:end])
	}
	ingestDur := time.Since(start)
	memAfter := memMB()
	t.Logf("BADGER Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())
	t.Logf("BADGER RAM: %.1f MB before → %.1f MB after (delta %.1f MB)", memBefore, memAfter, memAfter-memBefore)

	// Len
	start = time.Now()
	n := s.Len(nil)
	t.Logf("BADGER Len: %d in %v", n, time.Since(start))

	// Full scan
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	t.Logf("BADGER Full scan: %d triples in %v", count, time.Since(start))

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("BADGER Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count = 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("BADGER Predicate scan (p42): %d triples in %v", count, time.Since(start))
}

func TestStress1M_BadgerDisk(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)
	dir := t.TempDir()

	s, err := badgerstore.New(badgerstore.WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Ingest
	start := time.Now()
	batch := 50_000
	for i := 0; i < len(quads); i += batch {
		end := i + batch
		if end > len(quads) {
			end = len(quads)
		}
		s.AddN(quads[i:end])
	}
	ingestDur := time.Since(start)
	t.Logf("BADGER-DISK Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("BADGER-DISK Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("BADGER-DISK Predicate scan (p42): %d triples in %v", count, time.Since(start))
}

func TestStress1M_SQLite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)

	s, err := sqlitestore.New(sqlitestore.WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	memBefore := memMB()

	// Ingest in batches
	start := time.Now()
	batch := 50_000
	for i := 0; i < len(quads); i += batch {
		end := i + batch
		if end > len(quads) {
			end = len(quads)
		}
		s.AddN(quads[i:end])
	}
	ingestDur := time.Since(start)
	memAfter := memMB()
	t.Logf("SQLITE Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())
	t.Logf("SQLITE RAM: %.1f MB before → %.1f MB after (delta %.1f MB)", memBefore, memAfter, memAfter-memBefore)

	// Len
	start = time.Now()
	n := s.Len(nil)
	t.Logf("SQLITE Len: %d in %v", n, time.Since(start))

	// Full scan
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	t.Logf("SQLITE Full scan: %d triples in %v", count, time.Since(start))

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("SQLITE Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count = 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("SQLITE Predicate scan (p42): %d triples in %v", count, time.Since(start))
}

func TestStress1M_SQLiteDisk(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)
	path := t.TempDir() + "/stress.db"

	s, err := sqlitestore.New(sqlitestore.WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Ingest
	start := time.Now()
	batch := 50_000
	for i := 0; i < len(quads); i += batch {
		end := i + batch
		if end > len(quads) {
			end = len(quads)
		}
		s.AddN(quads[i:end])
	}
	ingestDur := time.Since(start)
	t.Logf("SQLITE-DISK Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("SQLITE-DISK Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("SQLITE-DISK Predicate scan (p42): %d triples in %v", count, time.Since(start))
}
