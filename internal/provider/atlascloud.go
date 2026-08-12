package provider

const defaultAtlasCloudBaseURL = "https://api.atlascloud.ai/v1"

// AtlasCloud is an OpenAI-compatible provider configured for Atlas Cloud.
type AtlasCloud struct {
	*OpenAI
}

// NewAtlasCloud creates an Atlas Cloud provider using its default API endpoint.
func NewAtlasCloud(apiKey string, baseURL string) (*AtlasCloud, error) {
	if baseURL == "" {
		baseURL = defaultAtlasCloudBaseURL
	}

	openAI, err := NewOpenAI(apiKey, WithOpenAIBaseURL(baseURL))
	if err != nil {
		return nil, err
	}

	return &AtlasCloud{OpenAI: openAI}, nil
}
