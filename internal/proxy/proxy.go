package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TokenUsage represents the usage field from a chat completion response.
// Standard OpenAI-compatible format used by DeepSeek.
type TokenUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// ChatCompletionResponse is the partial response we parse for usage.
type ChatCompletionResponse struct {
	Model   string     `json:"model"`
	Usage   *TokenUsage `json:"usage,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// TokenRecord is stored in the token_usage table.
type TokenRecord struct {
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	Model            string    `json:"model"`
	CreatedAt        time.Time `json:"created_at"`
}

// TokenStore defines the interface for persisting token records.
type TokenStore interface {
	SaveTokenRecord(r *TokenRecord) error
}

// Proxy is a reverse proxy that intercepts chat completion API calls.
// It forwards requests to the upstream provider (e.g., DeepSeek),
// intercepts the response to extract token usage, then returns the
// response to the caller unchanged.
type Proxy struct {
	mu         sync.RWMutex
	upstream   string // e.g., https://api.deepseek.com
	apiKey     string
	server     *http.Server
	store      TokenStore
	running    bool
	client     *http.Client
	stats      ProxyStats
}

// ProxyStats holds running counters.
type ProxyStats struct {
	TotalRequests   int64
	TotalPrompt     int64
	TotalCompletion int64
	TotalTokens     int64
}

// New creates a proxy that forwards to the given upstream URL.
func New(upstream, apiKey string, store TokenStore) *Proxy {
	return &Proxy{
		upstream: strings.TrimRight(upstream, "/"),
		apiKey:   apiKey,
		store:    store,
		client: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 90 * time.Second,
			},
		},
	}
}

// Start begins listening on the given address.
func (p *Proxy) Start(addr string) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("already running")
	}
	p.running = true
	p.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", p.handleChatCompletion)
	mux.HandleFunc("/v1/", p.handleForward)
	mux.HandleFunc("/health", p.handleHealth)

	p.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("[proxy] listening on %s, upstream: %s\n", addr, p.upstream)
	return p.server.ListenAndServe()
}

// Stop shuts down the proxy server.
func (p *Proxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.server != nil {
		p.server.Close()
	}
	p.running = false
}

// Stats returns a copy of current statistics.
func (p *Proxy) Stats() ProxyStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// handleChatCompletion intercepts the /v1/chat/completions endpoint.
func (p *Proxy) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	// Read request body for forwarding
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build upstream request
	upstreamURL := p.upstream + "/v1/chat/completions"
	upReq, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, io.NopCloser(strings.NewReader(string(body))))
	if err != nil {
		http.Error(w, "create request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for k, v := range r.Header {
		upReq.Header[k] = v
	}
	// Ensure auth header
	upReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	upReq.Header.Set("Content-Type", "application/json")

	// Forward request
	upResp, err := p.client.Do(upReq)
	if err != nil {
		http.Error(w, "upstream: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer upResp.Body.Close()

	// Read upstream response body
	respBody, err := io.ReadAll(upResp.Body)
	if err != nil {
		http.Error(w, "read upstream: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse usage from response (best-effort, don't block response)
	go p.parseAndStoreUsage(respBody)

	// Copy upstream response headers
	for k, v := range upResp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(upResp.StatusCode)
	w.Write(respBody)
}

// handleForward forwards all other /v1/* requests.
func (p *Proxy) handleForward(w http.ResponseWriter, r *http.Request) {
	upstreamURL := p.upstream + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	body, _ := io.ReadAll(r.Body)
	upReq, _ := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, io.NopCloser(strings.NewReader(string(body))))
	for k, v := range r.Header {
		upReq.Header[k] = v
	}
	upReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	upResp, err := p.client.Do(upReq)
	if err != nil {
		http.Error(w, "upstream: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer upResp.Body.Close()

	respBody, _ := io.ReadAll(upResp.Body)
	for k, v := range upResp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(upResp.StatusCode)
	w.Write(respBody)
}

// handleHealth returns proxy health status.
func (p *Proxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := p.Stats()
	fmt.Fprintf(w, "{\"status\":\"ok\",\"requests\":%d,\"tokens\":%d}",
		stats.TotalRequests, stats.TotalTokens)
}

// parseAndStoreUsage extracts token usage from a chat completion response.
func (p *Proxy) parseAndStoreUsage(body []byte) {
	var resp ChatCompletionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return // Not a valid JSON response or no usage field
	}
	if resp.Usage == nil {
		return // No usage data in this response
	}
	if resp.Error != nil {
		return // Error response, no usage
	}

	record := &TokenRecord{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
		Model:            resp.Model,
		CreatedAt:        time.Now(),
	}

	p.mu.Lock()
	p.stats.TotalRequests++
	p.stats.TotalPrompt += record.PromptTokens
	p.stats.TotalCompletion += record.CompletionTokens
	p.stats.TotalTokens += record.TotalTokens
	p.mu.Unlock()

	if p.store != nil {
		if err := p.store.SaveTokenRecord(record); err != nil {
			fmt.Printf("[proxy] save token record: %v\n", err)
		}
	}
}
