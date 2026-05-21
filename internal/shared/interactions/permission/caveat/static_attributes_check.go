package caveat

type StaticAttributesCheckParam struct {
	AllowedTypes       []string `json:"allowed_types"`
	AllowedLevels      []string `json:"allowed_levels"`
	IsContainSecretary bool     `json:"is_contain_secretary"`
}

type StaticAttributesCheckOption func(*StaticAttributesCheckParam)

func WithAllowedTypes(allowedTypes []string) StaticAttributesCheckOption {
	return func(param *StaticAttributesCheckParam) {
		param.AllowedTypes = allowedTypes
	}
}

func WithAllowedLevels(allowedLevels []string) StaticAttributesCheckOption {
	return func(param *StaticAttributesCheckParam) {
		param.AllowedLevels = allowedLevels
	}
}

func WithIsContainSecretary(isContainSecretary bool) StaticAttributesCheckOption {
	return func(param *StaticAttributesCheckParam) {
		param.IsContainSecretary = isContainSecretary
	}
}

func NewStaticAttributesCheck(options ...StaticAttributesCheckOption) *Caveat {
	params := &StaticAttributesCheckParam{}
	for _, opt := range options {
		opt(params)
	}
	return &Caveat{
		Name:    "static_attributes_check",
		Context: *params,
	}
}
