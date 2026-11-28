package provider

const defaultLLMTRBaseURL = "https://llmtr.com/v1"

// LLMTR is an OpenAI-compatible provider configured for LLMTR.
type LLMTR struct {
	*OpenAI
}

// NewLLMTR creates a LLMTR provider, using its default endpoint when baseURL is empty.
// @anchor:provider:entry Configure the LLMTR endpoint and require its API key.
func NewLLMTR(apiKey string, baseURL string) (*LLMTR, error) {
	if apiKey == "" {
		return nil, &ProviderError{
			Category: ErrCategoryAuth,
			Message:  "LLMTR API key is required; set LLMTR_API_KEY",
		}
	}

	if baseURL == "" {
		baseURL = defaultLLMTRBaseURL
	}

	openAI, err := NewOpenAI(apiKey, WithOpenAIBaseURL(baseURL))
	if err != nil {
		return nil, err
	}

	return &LLMTR{OpenAI: openAI}, nil
}
