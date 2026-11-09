package provider

const defaultStandardComputeBaseURL = "https://api.stdcmpt.com/v1"

// StandardCompute is an OpenAI-compatible provider configured for Standard Compute.
type StandardCompute struct {
	*OpenAI
}

// NewStandardCompute creates a Standard Compute provider using its default API endpoint.
func NewStandardCompute(apiKey string, baseURL string) (*StandardCompute, error) {
	if apiKey == "" {
		return nil, &ProviderError{
			Category: ErrCategoryAuth,
			Message:  "Standard Compute API key is required; set STANDARDCOMPUTE_API_KEY",
		}
	}

	if baseURL == "" {
		baseURL = defaultStandardComputeBaseURL
	}

	openAI, err := NewOpenAI(apiKey, WithOpenAIBaseURL(baseURL))
	if err != nil {
		return nil, err
	}

	return &StandardCompute{OpenAI: openAI}, nil
}
