package handler

import "sort"

// MapValidationErrors validates input and returns the failed fields sorted by
// name, or nil when input is valid.
func (b *BaseHandler) MapValidationErrors(input any) []FieldError {
	validationErrors := b.Validator.ValidateStruct(input)
	if len(validationErrors) == 0 {
		return nil
	}
	result := make([]FieldError, 0, len(validationErrors))
	for field, message := range validationErrors {
		result = append(result, FieldError{Field: field, Message: message})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Field < result[j].Field })
	return result
}
