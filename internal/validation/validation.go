package validation

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

type Validator struct{ v *validator.Validate }

func NewValidator() *Validator {
	v := validator.New()
	// Custom validations can be registered here
	return &Validator{v: v}
}

func (vv *Validator) ValidateStruct(s any) map[string]string {
	if err := vv.v.Struct(s); err != nil {
		var vErrors validator.ValidationErrors
		if errors.As(err, &vErrors) {
			out := make(map[string]string, len(vErrors))
			for _, fe := range vErrors {
				out[fe.Field()] = fmt.Sprintf("%s", fe.Tag())
			}
			return out
		}
		return map[string]string{"_error": err.Error()}
	}
	return nil
}

func (vv *Validator) Engine() *validator.Validate { return vv.v }
