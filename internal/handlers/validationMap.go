package handlers

func (b *BaseHandler) MapValidationErrors(input any) []FieldError {
	validationErrors := b.Validator.ValidateStruct(input)
	if len(validationErrors) == 0 {
		return nil
	}
	result := make([]FieldError, 0, len(validationErrors))
	for field, message := range validationErrors {
		result = append(result, FieldError{Field: field, Message: message})
	}
	return result
}
