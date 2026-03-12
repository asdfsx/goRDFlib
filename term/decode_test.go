package term

import "testing"

func TestTermFromKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string // N3 representation
	}{
		{"URIRef", "U:http://example.org/x", "<http://example.org/x>"},
		{"BNode", "B:b1", "_:b1"},
		{"plain literal", `L:"hello"`, `"hello"`},
		{"lang literal", `L:"bonjour"@fr`, `"bonjour"@fr`},
		{"dir literal", `L:"hello"@en--ltr`, `"hello"@en--ltr`},
		{"typed literal", `L:"42"^^<http://www.w3.org/2001/XMLSchema#integer>`, "42"},
		{"integer shorthand", "L:42", "42"},
		{"boolean shorthand", "L:true", "true"},
		{"decimal shorthand", "L:3.14", "3.14"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term, err := TermFromKey(tt.key)
			if err != nil {
				t.Fatalf("TermFromKey(%q): %v", tt.key, err)
			}
			if got := term.N3(); got != tt.want {
				t.Errorf("N3() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTermFromKeyErrors(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"too short", "X"},
		{"unknown prefix", "Z:foo"},
		{"bad literal", `L:bad`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := TermFromKey(tt.key)
			if err == nil {
				t.Errorf("expected error for key %q", tt.key)
			}
		})
	}
}

func TestTermRoundTrip(t *testing.T) {
	terms := []Term{
		NewURIRefUnsafe("http://example.org/test"),
		NewBNode("node1"),
		NewLiteral("hello"),
		NewLiteral("bonjour", WithLang("fr")),
		NewLiteral("hello", WithLang("en"), WithDir("ltr")),
		NewLiteral(42),
		NewLiteral(3.14),
		NewLiteral(true),
		NewLiteral("text with \"quotes\""),
		NewLiteral("line1\nline2"),
	}
	for _, original := range terms {
		key := TermKey(original)
		decoded, err := TermFromKey(key)
		if err != nil {
			t.Errorf("TermFromKey(%q): %v", key, err)
			continue
		}
		if !original.Equal(decoded) {
			t.Errorf("round-trip failed: %s → %q → %s", original.N3(), key, decoded.N3())
		}
	}
}
