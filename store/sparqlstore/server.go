package sparqlstore

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// Server is an HTTP handler that implements the W3C SPARQL 1.1 Protocol
// backed by an in-memory dataset. Intended for testing.
//
// Server is not safe for concurrent use when the dataset is mutated (e.g.,
// concurrent query and update requests may race on Dataset.NamedGraphs).
// For production use, wrap with external synchronization.
//
// Reference: https://www.w3.org/TR/sparql11-protocol/
type Server struct {
	dataset *sparql.Dataset
}

// NewServer creates a Server backed by the given dataset.
func NewServer(ds *sparql.Dataset) *Server {
	return &Server{dataset: ds}
}

// NewServerWithGraph creates a Server backed by a single default graph.
func NewServerWithGraph(g *rdflibgo.Graph) *Server {
	return &Server{
		dataset: &sparql.Dataset{
			Default:     g,
			NamedGraphs: make(map[string]*rdflibgo.Graph),
		},
	}
}

// QueryHandler returns an http.Handler for the SPARQL query endpoint.
//
// Supports:
//   - POST with application/x-www-form-urlencoded (query parameter)
//   - POST with application/sparql-query (body is the query)
//   - GET with ?query= parameter
func (s *Server) QueryHandler() http.Handler {
	return http.HandlerFunc(s.handleQuery)
}

// UpdateHandler returns an http.Handler for the SPARQL update endpoint.
//
// Supports:
//   - POST with application/sparql-update (body is the update)
//   - POST with application/x-www-form-urlencoded (update parameter)
func (s *Server) UpdateHandler() http.Handler {
	return http.HandlerFunc(s.handleUpdate)
}

// Handler returns a combined handler that routes /query and /update paths.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/query", s.QueryHandler())
	mux.Handle("/update", s.UpdateHandler())
	return mux
}

// handleQuery processes SPARQL query requests. It extracts the query string
// from GET parameters, POST form data, or POST body (application/sparql-query),
// builds the effective dataset from protocol parameters, executes the query,
// and writes the result in the format requested by the Accept header.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var queryStr string

	switch r.Method {
	case http.MethodGet:
		queryStr = r.URL.Query().Get("query")
	case http.MethodPost:
		ct := r.Header.Get("Content-Type")
		switch {
		case strings.Contains(ct, "application/x-www-form-urlencoded"):
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form: "+err.Error(), http.StatusBadRequest)
				return
			}
			queryStr = r.FormValue("query")
		case strings.Contains(ct, "application/sparql-query"):
			body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MB max
			if err != nil {
				http.Error(w, "read error: "+err.Error(), http.StatusBadRequest)
				return
			}
			queryStr = string(body)
		default:
			http.Error(w, "unsupported content type: "+ct, http.StatusUnsupportedMediaType)
			return
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if queryStr == "" {
		http.Error(w, "missing query parameter", http.StatusBadRequest)
		return
	}

	// Handle protocol-specified dataset parameters.
	defaultGraphURIs := r.URL.Query()["default-graph-uri"]
	namedGraphURIs := r.URL.Query()["named-graph-uri"]
	if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if dg := r.Form["default-graph-uri"]; len(dg) > 0 {
			defaultGraphURIs = dg
		}
		if ng := r.Form["named-graph-uri"]; len(ng) > 0 {
			namedGraphURIs = ng
		}
	}

	// Build effective dataset if protocol params are specified.
	ds := s.dataset
	if len(defaultGraphURIs) > 0 || len(namedGraphURIs) > 0 {
		ds = s.buildProtocolDataset(defaultGraphURIs, namedGraphURIs)
	}

	// NOTE: Queries run against ds.Default only. GRAPH clause queries against
	// named graphs in the dataset are not supported by the test server.
	// For full named graph support, use a SPARQL engine with dataset awareness.
	result, err := sparql.Query(ds.Default, queryStr)
	if err != nil {
		http.Error(w, "query error: "+err.Error(), http.StatusBadRequest)
		return
	}

	accept := r.Header.Get("Accept")
	switch {
	case result.Type == "CONSTRUCT":
		s.writeConstructResult(w, result, accept)
	default:
		s.writeSelectResult(w, result, accept)
	}
}

// handleUpdate processes SPARQL update requests. It extracts the update string
// from the POST body (application/sparql-update) or form data, executes it
// against the dataset, and returns 200 OK on success.
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var updateStr string
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "application/sparql-update"):
		body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MB max
		if err != nil {
			http.Error(w, "read error: "+err.Error(), http.StatusBadRequest)
			return
		}
		updateStr = string(body)
	case strings.Contains(ct, "application/x-www-form-urlencoded"):
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form: "+err.Error(), http.StatusBadRequest)
			return
		}
		updateStr = r.FormValue("update")
	default:
		http.Error(w, "unsupported content type: "+ct, http.StatusUnsupportedMediaType)
		return
	}

	if updateStr == "" {
		http.Error(w, "missing update", http.StatusBadRequest)
		return
	}

	if err := sparql.Update(s.dataset, updateStr); err != nil {
		http.Error(w, "update error: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// writeSelectResult serializes a SELECT or ASK result as XML or JSON
// depending on the Accept header. JSON is used when the header contains "json",
// otherwise XML is the default.
func (s *Server) writeSelectResult(w http.ResponseWriter, result *sparql.Result, accept string) {
	if strings.Contains(accept, "json") {
		w.Header().Set("Content-Type", "application/sparql-results+json")
		s.writeResultJSON(w, result)
		return
	}
	w.Header().Set("Content-Type", "application/sparql-results+xml")
	s.writeResultXML(w, result)
}

// writeConstructResult serializes a CONSTRUCT result as N-Triples.
// If the result graph is nil, an empty response with the N-Triples content
// type is returned.
func (s *Server) writeConstructResult(w http.ResponseWriter, result *sparql.Result, accept string) {
	if result.Graph == nil {
		w.Header().Set("Content-Type", "application/n-triples")
		return
	}
	w.Header().Set("Content-Type", "application/n-triples")
	_ = nt.Serialize(result.Graph, w)
}

// writeResultXML serializes a SPARQL result as application/sparql-results+xml.
// For ASK queries it writes a <boolean> element; for SELECT queries it writes
// <head> with variable declarations followed by <results> with binding rows.
func (s *Server) writeResultXML(w http.ResponseWriter, result *sparql.Result) {
	type xmlBinding struct {
		XMLName xml.Name `xml:"binding"`
		Name    string   `xml:"name,attr"`
		URI     string   `xml:"uri,omitempty"`
		BNode   string   `xml:"bnode,omitempty"`
		Literal *xmlLit  `xml:"literal,omitempty"`
	}
	type xmlResult struct {
		XMLName  xml.Name     `xml:"result"`
		Bindings []xmlBinding `xml:"binding"`
	}

	w.Write([]byte(xml.Header))
	w.Write([]byte(`<sparql xmlns="http://www.w3.org/2005/sparql-results#">` + "\n"))

	if result.Type == "ASK" {
		w.Write([]byte("<head/>\n"))
		if result.AskResult {
			w.Write([]byte("<boolean>true</boolean>\n"))
		} else {
			w.Write([]byte("<boolean>false</boolean>\n"))
		}
		w.Write([]byte("</sparql>\n"))
		return
	}

	w.Write([]byte("<head>"))
	for _, v := range result.Vars {
		fmt.Fprintf(w, `<variable name="%s"/>`, v)
	}
	w.Write([]byte("</head>\n<results>\n"))

	for _, row := range result.Bindings {
		xr := xmlResult{}
		for _, v := range result.Vars {
			t, ok := row[v]
			if !ok || t == nil {
				continue
			}
			xb := xmlBinding{Name: v}
			switch val := t.(type) {
			case term.URIRef:
				xb.URI = val.Value()
			case term.BNode:
				xb.BNode = val.Value()
			case term.Literal:
				xl := &xmlLit{Value: val.Lexical()}
				if val.Language() != "" {
					xl.Lang = val.Language()
					if val.Dir() != "" {
						xl.Lang += "--" + val.Dir()
					}
				} else if dt := val.Datatype(); dt != term.XSDString {
					xl.Datatype = dt.Value()
				}
				xb.Literal = xl
			}
			xr.Bindings = append(xr.Bindings, xb)
		}
		data, _ := xml.Marshal(xr)
		w.Write(data)
		w.Write([]byte("\n"))
	}
	w.Write([]byte("</results>\n</sparql>\n"))
}

// xmlLit represents a literal value in the SPARQL XML results format,
// with optional language tag and datatype attributes.
type xmlLit struct {
	XMLName  xml.Name `xml:"literal"`
	Value    string   `xml:",chardata"`
	Lang     string   `xml:"http://www.w3.org/XML/1998/namespace lang,attr,omitempty"`
	Datatype string   `xml:"datatype,attr,omitempty"`
}

// writeResultJSON serializes a SPARQL result as application/sparql-results+json.
// For ASK queries it writes {"boolean": true/false}; for SELECT queries it
// writes the standard JSON results format with head.vars and results.bindings.
func (s *Server) writeResultJSON(w http.ResponseWriter, result *sparql.Result) {
	if result.Type == "ASK" {
		if result.AskResult {
			w.Write([]byte(`{"boolean":true}`))
		} else {
			w.Write([]byte(`{"boolean":false}`))
		}
		return
	}

	w.Write([]byte(`{"head":{"vars":[`))
	for i, v := range result.Vars {
		if i > 0 {
			w.Write([]byte(","))
		}
		fmt.Fprintf(w, `"%s"`, v)
	}
	w.Write([]byte(`]},"results":{"bindings":[`))

	for i, row := range result.Bindings {
		if i > 0 {
			w.Write([]byte(","))
		}
		w.Write([]byte("{"))
		first := true
		for _, v := range result.Vars {
			t, ok := row[v]
			if !ok || t == nil {
				continue
			}
			if !first {
				w.Write([]byte(","))
			}
			first = false
			fmt.Fprintf(w, `"%s":`, v)
			s.writeTermJSON(w, t)
		}
		w.Write([]byte("}"))
	}
	w.Write([]byte("]}}\n"))
}

// writeTermJSON writes a single RDF term as a JSON object with "type" and
// "value" fields, plus optional "xml:lang" or "datatype" for literals.
func (s *Server) writeTermJSON(w io.Writer, t rdflibgo.Term) {
	switch val := t.(type) {
	case term.URIRef:
		fmt.Fprintf(w, `{"type":"uri","value":"%s"}`, escapeJSON(val.Value()))
	case term.BNode:
		fmt.Fprintf(w, `{"type":"bnode","value":"%s"}`, escapeJSON(val.Value()))
	case term.Literal:
		fmt.Fprintf(w, `{"type":"literal","value":"%s"`, escapeJSON(val.Lexical()))
		if val.Language() != "" {
			lang := val.Language()
			if val.Dir() != "" {
				lang += "--" + val.Dir()
			}
			fmt.Fprintf(w, `,"xml:lang":"%s"`, lang)
		} else if dt := val.Datatype(); dt != term.XSDString {
			fmt.Fprintf(w, `,"datatype":"%s"`, escapeJSON(dt.Value()))
		}
		w.Write([]byte("}"))
	}
}

// escapeJSON escapes a string for embedding in a JSON value using json.Marshal,
// stripping the surrounding quotes from the marshaled output.
func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// buildProtocolDataset constructs a dataset from protocol-specified graph URIs.
func (s *Server) buildProtocolDataset(defaultGraphURIs, namedGraphURIs []string) *sparql.Dataset {
	ds := &sparql.Dataset{
		Default:     graph.NewGraph(graph.WithStore(store.NewMemoryStore())),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}

	// Merge specified default graphs.
	for _, uri := range defaultGraphURIs {
		if g, ok := s.dataset.NamedGraphs[uri]; ok {
			for t := range g.Triples(nil, nil, nil) {
				ds.Default.Add(t.Subject, t.Predicate, t.Object)
			}
		}
	}

	// Copy specified named graphs.
	for _, uri := range namedGraphURIs {
		if g, ok := s.dataset.NamedGraphs[uri]; ok {
			ng := graph.NewGraph(graph.WithStore(store.NewMemoryStore()))
			for t := range g.Triples(nil, nil, nil) {
				ng.Add(t.Subject, t.Predicate, t.Object)
			}
			ds.NamedGraphs[uri] = ng
		}
	}

	return ds
}

// WriteNTriples serializes a graph to an io.Writer in N-Triples format.
// Exposed for test utilities that need to compare graph contents.
func WriteNTriples(g *rdflibgo.Graph, w io.Writer) error {
	return nt.Serialize(g, w)
}
