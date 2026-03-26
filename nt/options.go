package nt

// Option configures N-Triples parsing or serialization.
type Option func(*config)

// ErrorHandler is called when a line fails to parse.
// It receives the 1-based line number, the raw line text, and the parse error.
// If retry is true, fixedLine is parsed instead (exactly one retry attempt).
// If retry is false, the line is skipped and parsing continues.
// To preserve the default fail-fast behavior, do not set an error handler.
type ErrorHandler func(lineNum int, line string, err error) (fixedLine string, retry bool)

type config struct {
	base         string
	errorHandler ErrorHandler
}

// WithBase sets the base IRI for resolving relative IRIs.
func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}

// WithErrorHandler sets a callback invoked when a line fails to parse.
// See ErrorHandler for semantics.
func WithErrorHandler(h ErrorHandler) Option {
	return func(c *config) { c.errorHandler = h }
}
