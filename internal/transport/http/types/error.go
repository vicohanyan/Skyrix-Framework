package types

type HttpValidationError struct {
	Field string `json:"field"`
	Tag   string `json:"tag"`
}

type HttpValidationErrors struct {
	Errors []HttpValidationError `json:"apiErrors"`
}

type ErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}
