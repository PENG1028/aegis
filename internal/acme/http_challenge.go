package acme

import "sync"

// HTTPChallengeProvider exposes lego HTTP-01 tokens through the existing Aegis
// HTTP server. Caddy proxies the well-known challenge path to that server.
type HTTPChallengeProvider struct {
	mu     sync.RWMutex
	tokens map[string]string
}

func newHTTPChallengeProvider() *HTTPChallengeProvider {
	return &HTTPChallengeProvider{tokens: make(map[string]string)}
}

func (p *HTTPChallengeProvider) Present(_, token, keyAuth string) error {
	p.mu.Lock()
	p.tokens[token] = keyAuth
	p.mu.Unlock()
	return nil
}

func (p *HTTPChallengeProvider) CleanUp(_, token, _ string) error {
	p.mu.Lock()
	delete(p.tokens, token)
	p.mu.Unlock()
	return nil
}

func (p *HTTPChallengeProvider) Response(token string) (string, bool) {
	p.mu.RLock()
	value, ok := p.tokens[token]
	p.mu.RUnlock()
	return value, ok
}
