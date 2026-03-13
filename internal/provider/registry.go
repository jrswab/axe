package provider

import "fmt"

// supportedProviders lists all provider names accepted by New.
var supportedProviders = map[string]bool{
	"anthropic": true,
	"openai":    true,
	"ollama":    true,
	"opencode":  true,
	"google":    true,
	"minimax":   true,
}

// Supported reports whether providerName is a known provider.
func Supported(providerName string) bool {
	return supportedProviders[providerName]
}

// New creates a Provider by dispatching to the correct constructor based on providerName.
func New(providerName, apiKey, baseURL string) (Provider, error) {
	switch providerName {
	case "anthropic":
		var opts []AnthropicOption
		if baseURL != "" {
			opts = append(opts, WithBaseURL(baseURL))
		}
		return NewAnthropic(apiKey, opts...)

	case "openai":
		var opts []OpenAIOption
		if baseURL != "" {
			opts = append(opts, WithOpenAIBaseURL(baseURL))
		}
		return NewOpenAI(apiKey, opts...)

	case "ollama":
		var opts []OllamaOption
		if baseURL != "" {
			opts = append(opts, WithOllamaBaseURL(baseURL))
		}
		return NewOllama(opts...)

	case "opencode":
		var opts []OpenCodeOption
		if baseURL != "" {
			opts = append(opts, WithOpenCodeBaseURL(baseURL))
		}
		return NewOpenCode(apiKey, opts...)

	case "google":
		var opts []GeminiOption
		if baseURL != "" {
			opts = append(opts, WithGeminiBaseURL(baseURL))
		}
		return NewGemini(apiKey, opts...)

	case "minimax":
		var opts []AnthropicOption
		if baseURL != "" {
			opts = append(opts, WithBaseURL(baseURL))
		}
		return NewMiniMax(apiKey, opts...)

	default:
		return nil, fmt.Errorf("unsupported provider %q: supported providers are anthropic, openai, ollama, opencode, google, minimax", providerName)
	}
}
