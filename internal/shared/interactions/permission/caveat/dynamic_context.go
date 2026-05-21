package caveat

type DynamicContextParam struct {
	AllowRAS bool `json:"allow_ras"`
}

func NewDynamicContext(params DynamicContextParam) *Caveat {
	return &Caveat{
		Name:    "enable_dynamic_context",
		Context: params,
	}
}
