package llm

type Option func(*LLMConfig)

func WithBaseURL(url string) Option {
	return func(o *LLMConfig) {
		o.BaseURL = url
	}
}

func WithModel(model string) Option {
	return func(o *LLMConfig) {
		o.Model = model
	}
}

func WithAPIKey(key string) Option {
	return func(o *LLMConfig) {
		o.APIKey = key
	}
}
