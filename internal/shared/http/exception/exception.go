package exception

type Exception struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

type ErrorResponse struct {
	Error Exception `json:"error"`
}

type Option func(*Exception)

func New(code, message string, opts ...Option) Exception {
	ex := Exception{Code: code, Message: message}
	for _, opt := range opts {
		if opt != nil {
			opt(&ex)
		}
	}
	return ex
}

func WithDetails(details map[string]any) Option {
	return func(ex *Exception) {
		ex.Details = details
	}
}

func WithRequestId(requestID string) Option {
	return func(ex *Exception) {
		ex.RequestID = requestID
	}
}

func WrapResponse(ex Exception) ErrorResponse {
	return ErrorResponse{Error: ex}
}
