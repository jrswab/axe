package provider

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
func NewEdenAI(apiKey string, baseURL string) (*EdenAI, error) {
	if apiKey == "" {
		return nil, &ProviderError{
			Category: ErrCategoryAuth,
			Message:  "Eden AI API key is required; set EDENAI_API_KEY",
		}
	}

	if baseURL == "" {
		baseURL = defaultEdenAIBaseURL
	}

	openAI, err := NewOpenAI(apiKey, WithOpenAIBaseURL(baseURL))
	if err != nil {
		return nil, err
	}

	return &EdenAI{OpenAI: openAI}, nil
}
