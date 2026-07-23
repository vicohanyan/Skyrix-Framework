package schemaResolver

import (
	"encoding/json"
	"errors"
	"net/http"
)

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type errorPayload struct {
	Error errorBody `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorPayload{Error: errorBody{Code: code, Message: msg}})
}

func HTTPError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrTenantHeaderMissing):
		writeJSON(w, http.StatusBadRequest, "TENANT_REQUIRED", "Missing tenant/domain")
	case errors.Is(err, ErrTenantInvalid):
		writeJSON(w, http.StatusBadRequest, "TENANT_INVALID", "Invalid tenant")
	case errors.Is(err, ErrTenantNotFound):
		writeJSON(w, http.StatusNotFound, "TENANT_NOT_FOUND", "Tenant not found")
	case errors.Is(err, ErrTenantNotFoundHost):
		writeJSON(w, http.StatusNotFound, "TENANT_NOT_FOUND_BY_DOMAIN", "Tenant not found for this host")
	case errors.Is(err, ErrHostEmpty):
		writeJSON(w, http.StatusBadRequest, "HOST_EMPTY", "Empty host")
	case errors.Is(err, ErrSchemaInvalid):
		writeJSON(w, http.StatusInternalServerError, "SCHEMA_INVALID", "Invalid database schema")
	default:
		writeJSON(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
	}
}
