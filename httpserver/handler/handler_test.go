package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/skyrix-lib/framework/validation"
)

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) ErrorPayload {
	t.Helper()
	var payload ErrorPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload %q: %v", recorder.Body.String(), err)
	}
	return payload
}

func TestDecodeJSONRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	base := &BaseHandler{}
	var dst struct {
		Name string `json:"name"`
	}

	for body, wantMessage := range map[string]string{
		`{"name":"a","extra":1}`: "Invalid JSON payload",
		`{"name":"a"} {}`:        "Unexpected trailing data",
		``:                       "Invalid JSON payload",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		if base.DecodeJSON(recorder, request, &dst, 0) {
			t.Fatalf("DecodeJSON(%q) = true", body)
		}
		payload := decodeError(t, recorder)
		if recorder.Code != http.StatusBadRequest || payload.Error.Code != ErrCodeValidation || payload.Error.Message != wantMessage {
			t.Fatalf("DecodeJSON(%q) = %d %+v", body, recorder.Code, payload)
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"}`))
	if !base.DecodeJSON(httptest.NewRecorder(), request, &dst, 0) || dst.Name != "ok" {
		t.Fatalf("DecodeJSON(valid) failed, dst = %+v", dst)
	}
}

func TestDecodeOptionalJSONAcceptsEmptyBody(t *testing.T) {
	var dst struct{}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	if !(&BaseHandler{}).DecodeOptionalJSON(httptest.NewRecorder(), request, &dst, 0) {
		t.Fatal("DecodeOptionalJSON(empty) = false")
	}
}

func TestHandleErrorWritesStableCode(t *testing.T) {
	for status, code := range map[int]string{
		http.StatusBadRequest:          ErrCodeValidation,
		http.StatusUnauthorized:        ErrCodeAuth,
		http.StatusForbidden:           ErrCodeForbidden,
		http.StatusNotFound:            ErrCodeNotFound,
		http.StatusConflict:            ErrCodeConflict,
		http.StatusInternalServerError: ErrCodeInternal,
	} {
		recorder := httptest.NewRecorder()
		(&BaseHandler{}).HandleError(recorder, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("cause"), "message", status)
		payload := decodeError(t, recorder)
		if recorder.Code != status || payload.Error.Code != code || payload.Error.Message != "message" {
			t.Fatalf("HandleError(%d) = %d %+v", status, recorder.Code, payload)
		}
	}
}

func TestMapValidationErrorsIsSortedByField(t *testing.T) {
	base := &BaseHandler{Validator: validation.NewValidator()}
	input := struct {
		Zeta  string `validate:"required"`
		Alpha string `validate:"required"`
	}{}
	errs := base.MapValidationErrors(input)
	if len(errs) != 2 || errs[0].Field != "Alpha" || errs[1].Field != "Zeta" || errs[0].Message != "required" {
		t.Fatalf("MapValidationErrors() = %+v", errs)
	}
	if valid := base.MapValidationErrors(struct{}{}); valid != nil {
		t.Fatalf("MapValidationErrors(valid) = %+v", valid)
	}
}
