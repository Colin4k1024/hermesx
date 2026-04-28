package auth

import "net/http"

// CredentialExtractor attempts to extract an AuthContext from a request.
// Returns nil, nil if this extractor does not apply (e.g. no matching header).
type CredentialExtractor interface {
	Extract(r *http.Request) (*AuthContext, error)
}

// ExtractorChain tries extractors in order, returning the first successful result.
type ExtractorChain struct {
	extractors []CredentialExtractor
}

func NewExtractorChain(extractors ...CredentialExtractor) *ExtractorChain {
	return &ExtractorChain{extractors: extractors}
}

// Add appends an extractor to the chain.
func (c *ExtractorChain) Add(e CredentialExtractor) {
	c.extractors = append(c.extractors, e)
}

// Extract iterates extractors until one returns a non-nil AuthContext.
// Returns nil, nil if no extractor matched (anonymous request).
func (c *ExtractorChain) Extract(r *http.Request) (*AuthContext, error) {
	for _, ex := range c.extractors {
		ac, err := ex.Extract(r)
		if err != nil {
			return nil, err
		}
		if ac != nil {
			return ac, nil
		}
	}
	return nil, nil
}
