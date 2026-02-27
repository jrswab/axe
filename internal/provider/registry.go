package provider

import "fmt"

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

	default:
		return nil, fmt.Errorf("unsupported provider %q: supported providers are anthropic, openai, ollama", providerName)
	}
}
