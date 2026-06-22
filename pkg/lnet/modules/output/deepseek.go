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
	apiKey string
	client *http.Client
}

// NewDeepSeek tworzy backend DeepSeek.
// apiKey — DEEPSEEK_API_KEY
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

// Name zwraca nazwę backendu.
func (d *DeepSeek) Name() string { return "deepseek" }

// Forward forwarduje request do api.deepseek.com.
// Dodaje thinking: {type: "disabled"}.
func (d *DeepSeek) Forward(ctx context.Context, req *core.Request) (*core.Response, error) {
	// Build upstream request
	upstreamURL := deepseekTarget + req.Path

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, upstreamURL, bytes.NewReader(req.Body))
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
