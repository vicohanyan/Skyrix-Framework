package schemaResolver

import (
	"net/http"
	"skyrix/internal/engine/tenantPackage/service"
	"strings"
)

type HeaderResolver struct {
	header string
	svc    *service.TenantService
}

func NewHeaderResolver(svc *service.TenantService, header string) *HeaderResolver {
	h := strings.TrimSpace(header)
	if h == "" {
		h = DefaultTenantHeader
	}
	return &HeaderResolver{header: h, svc: svc}
}

func (r *HeaderResolver) ResolveSchema(req *http.Request) (string, string, error) {
	tenant := strings.TrimSpace(req.Header.Get(r.header))
	if tenant == "" {
		return "", "", ErrTenantHeaderMissing
	}
	if !ReIdent.MatchString(tenant) {
		return "", "", ErrTenantInvalid
	}

	t, err := r.svc.GetByNamespace(req.Context(), tenant)
	if err != nil {
		return "", "", ErrTenantNotFound
	}

	if t.Schema == nil {
		return "", "", ErrSchemaInvalid
	}
	schema := strings.ToLower(strings.TrimSpace(*t.Schema))
	if !ReIdent.MatchString(schema) {
		return "", "", ErrSchemaInvalid
	}
	return schema, "header", nil
}
