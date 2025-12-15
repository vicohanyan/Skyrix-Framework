package handlers

import (
	"encoding/json"
	"io"
	"net/http"
)

func (b *BaseHandler) DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) bool {
	if maxBytes <= 0 {
		maxBytes = 1 << 20 // 1 MiB default
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		b.WriteJSON(w, http.StatusBadRequest, ErrorPayload{
			Error: ErrorBody{
				Code:    ErrCodeValidation,
				Message: "Invalid JSON payload",
				Details: []FieldError{{Field: "_", Message: classifyDecodeError(err)}},
			},
		})
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		b.WriteJSON(w, http.StatusBadRequest, ErrorPayload{
			Error: ErrorBody{
				Code:    ErrCodeValidation,
				Message: "Unexpected trailing data",
				Details: []FieldError{{Field: "_", Message: "unexpected_trailing_data"}},
			},
		})
		return false
	}
	return true
}

func classifyDecodeError(err error) string {
	if err == io.EOF {
		return "empty_body"
	}
	return "invalid_payload"
}
