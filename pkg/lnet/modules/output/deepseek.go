// output/deepseek.go — forward do api.deepseek.com + thinking injection.
//
// Implementuje Output + UsageExtractor.
// Dodaje thinking: {type: "disabled"} do każdego requestu.
// Zwraca Response ze streamującym body.

package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/QguAr71/sysq-libs/pkg/lnet/core"
	"github.com/QguAr71/sysq-libs/pkg/lnet/modules/common"
)

const deepseekTarget = "https://api.deepseek.com"

// DeepSeek — backend dla api.deepseek.com.
type DeepSeek struct {
	apiKey            string
	client            *http.Client
	thinkingDisabled  bool // inject thinking:{type:"disabled"} gdy true
}

// NewDeepSeek tworzy backend DeepSeek.
// thinking disabled — NIE wstrzykuje (V4 z reasoningiem).
func NewDeepSeek(apiKey string) *DeepSeek {
	return &DeepSeek{
		apiKey: apiKey,
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
				ResponseHeaderTimeout: 60 * time.Second,
			},
		},
	}
}

// NewDeepSeekWithThinkingDisabled tworzy backend DeepSeek z thinking:disabled.
// Używane przez eho1 (sandbox) — niższy koszt, bez reasoning_content.
func NewDeepSeekWithThinkingDisabled(apiKey string) *DeepSeek {
	d := NewDeepSeek(apiKey)
	d.thinkingDisabled = true
	return d
}

// Name zwraca nazwę backendu.
func (d *DeepSeek) Name() string { return "deepseek" }

// Forward forwarduje request do api.deepseek.com.
// Jeśli thinkingDisabled=true — dodaje thinking:{type:"disabled"} do JSON body.
// Jeśli thinkingDisabled=false — nie modyfikuje body (V4 z reasoningiem).
func (d *DeepSeek) Forward(ctx context.Context, req *core.Request) (*core.Response, error) {
	body := req.Body
	if d.thinkingDisabled && body != nil && len(body) > 0 {
		body = injectThinkingDisabled(body)
	}

	// Build upstream request
	upstreamURL := deepseekTarget + req.Path

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("deepseek: %w", err)
	}

	// Forward allowed headers + auth
	filtered := common.FilterHeaders(req.Headers)
	for k, v := range filtered {
		httpReq.Header[k] = v
	}
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)

	// Wykonaj
	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("deepseek upstream: %w", err)
	}

	return &core.Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       resp.Body,
	}, nil
}

// injectThinkingDisabled wstrzykuje thinking:{type:"disabled"} do JSON body.
// DeepSeek API v4 wymaga jawnego wyłączenia thinking.
func injectThinkingDisabled(body []byte) []byte {
	// jeśli body jest puste lub {"messages":[...]} — inject przed messages
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body // nie psuj jeśli nie JSON
	}
	if _, exists := raw["thinking"]; exists {
		return body // już jest
	}
	// Usuń messages, dodaj thinking, przywróć messages
	messagesJSON := raw["messages"]
	delete(raw, "messages")
	raw["thinking"] = json.RawMessage(`{"type":"disabled"}`)
	if messagesJSON != nil {
		raw["messages"] = messagesJSON
	}
	out, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	return out
}

// Extract implementuje UsageExtractor — parsuje usage z formatu DeepSeek.
func (d *DeepSeek) Extract(body []byte) *common.Usage {
	var resp struct {
		Usage struct {
			PromptTokensDetails struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			TotalTokens int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}
	return &common.Usage{
		CachedTokens: resp.Usage.PromptTokensDetails.CachedTokens,
		InputTokens:  resp.Usage.TotalTokens,
	}
}
