// Package translator provides format translation between different LLM API formats.
// Used by both the ai-platform server and the local node.
package translator

type RequestTranslator interface {
	TranslateRequest(req map[string]any) (map[string]any, error)
}

type ResponseTranslator interface {
	TranslateResponse(resp map[string]any) (map[string]any, error)
}

type Provider struct {
	Name     string
	Request  RequestTranslator
	Response ResponseTranslator
}

var registry = make(map[string]*Provider)

func Register(id string, p *Provider) {
	registry[id] = p
}

func Get(id string) *Provider {
	return registry[id]
}
