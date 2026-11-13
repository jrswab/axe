package provider

const defaultCheaperInferenceBaseURL = "https://api.cheaperinference.com/v1"

// CheaperInference is an OpenAI-compatible provider configured for Cheaper Inference.
type CheaperInference struct {
	*OpenAI
}

// NewCheaperInference creates a Cheaper Inference provider using its default API endpoint.
func NewCheaperInference(apiKey string, baseURL string) (*CheaperInference, error) {
	if apiKey == "" {
		return nil, &ProviderError{
			Category: ErrCategoryAuth,
			Message:  "Cheaper Inference API key is required; set CHEAPER_INFERENCE_API_KEY",
		}
	}

	if baseURL == "" {
		baseURL = defaultCheaperInferenceBaseURL
	}

	openAI, err := NewOpenAI(apiKey, WithOpenAIBaseURL(baseURL))
	if err != nil {
		return nil, err
	}

	return &CheaperInference{OpenAI: openAI}, nil
}
