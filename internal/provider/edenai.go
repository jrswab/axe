package provider

import (
	"net/netip"
	"net/url"
	"strings"
)

// defaultEdenAIBaseURL is Eden AI's default API endpoint. The base already
// includes the /v3 version segment, mirroring how the OpenAI default includes
// /v1.
const defaultEdenAIBaseURL = "https://api.edenai.run/v3"

// EdenAI is an OpenAI-compatible provider configured for Eden AI, an EU-based
// gateway that fronts several upstream vendors behind one key. Model ids are
// vendor-prefixed, for example edenai/openai/gpt-4o-mini: parseModel splits on
// the first slash, so edenai selects the provider and the remaining
// vendor/model is sent through unchanged.
type EdenAI struct {
	*OpenAI
}

// NewEdenAI creates an Eden AI provider using its default API endpoint.
// Custom endpoints require HTTPS, except localhost and loopback/private IP literals.
// @anchor:provider:entry Configure Eden AI credentials and validate the gateway endpoint before either request path can use it.
func NewEdenAI(apiKey string, baseURL string) (*EdenAI, error) {
	if apiKey == "" {
		return nil, &ProviderError{
			Category: ErrCategoryAuth,
			Message:  "Eden AI API key is required; set EDENAI_API_KEY",
		}
	}

	baseURL, err := normalizeEdenAIBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	openAI, err := NewOpenAI(apiKey, WithOpenAIBaseURL(baseURL))
	if err != nil {
		return nil, err
	}

	return &EdenAI{OpenAI: openAI}, nil
}

// normalizeEdenAIBaseURL validates the base before OpenAI appends the request path.
// @anchor:provider:policy Keep Eden AI credentials off public plaintext endpoints without DNS-based trust decisions.
func normalizeEdenAIBaseURL(baseURL string) (string, error) {
	if baseURL == "" {
		return defaultEdenAIBaseURL, nil
	}

	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" || u.Opaque != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", &ProviderError{
			Category: ErrCategoryBadRequest,
			Message:  "Eden AI base URL must be an absolute HTTP(S) URL with a host",
		}
	}
	if u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(baseURL, "#") {
		return "", &ProviderError{
			Category: ErrCategoryBadRequest,
			Message:  "Eden AI base URL must be without credentials, query, or fragment",
		}
	}

	if u.Scheme == "http" && !strings.EqualFold(u.Hostname(), "localhost") {
		ip, err := netip.ParseAddr(u.Hostname())
		if err != nil || (!ip.Unmap().IsLoopback() && !ip.Unmap().IsPrivate()) {
			return "", &ProviderError{
				Category: ErrCategoryBadRequest,
				Message:  "Eden AI base URL must use HTTPS; HTTP is allowed only for localhost or loopback/private IP literals",
			}
		}
	}
	return strings.TrimRight(baseURL, "/"), nil
}
