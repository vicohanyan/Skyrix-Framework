package handlers

func (b *BaseHandler) MapValidationErrors(dto any) []FieldError {
	m := b.Validator.ValidateStruct(dto)
	if len(m) == 0 {
		return nil
	}
	out := make([]FieldError, 0, len(m))
	for field, msg := range m {
		out = append(out, FieldError{Field: field, Message: msg})
	}
	return out
}
