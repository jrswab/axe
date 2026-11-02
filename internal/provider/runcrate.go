package provider

const defaultRuncrateBaseURL = "https://api.runcrate.ai/v1"

// Runcrate is an OpenAI-compatible provider configured for Runcrate.
type Runcrate struct {
	*OpenAI
}

// NewRuncrate creates a Runcrate provider using its default API endpoint.
func NewRuncrate(apiKey string, baseURL string) (*Runcrate, error) {
	if apiKey == "" {
		return nil, &ProviderError{
			Category: ErrCategoryAuth,
			Message:  "Runcrate API key is required; set RUNCRATE_API_KEY",
		}
	}

	if baseURL == "" {
		baseURL = defaultRuncrateBaseURL
	}

	openAI, err := NewOpenAI(apiKey, WithOpenAIBaseURL(baseURL))
	if err != nil {
		return nil, err
	}

	return &Runcrate{OpenAI: openAI}, nil
}
