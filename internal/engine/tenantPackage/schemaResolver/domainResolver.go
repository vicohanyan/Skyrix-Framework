package schemaResolver

import (
	"net/http"
	"skyrix/internal/engine/tenantPackage/service"
	"strings"
)

type DomainResolver struct {
	svc *service.TenantService
}

func NewDomainResolver(svc *service.TenantService) *DomainResolver {
	return &DomainResolver{svc: svc}
}

func (r *DomainResolver) ResolveSchema(req *http.Request) (string, string, error) {
	host := hostFromRequest(req)
	if host == "" {
		return "", "", ErrHostEmpty
	}

	t, err := r.svc.GetByDomain(req.Context(), host)
	if err != nil {
		return "", "", ErrTenantNotFoundHost
	}

	if t.Schema == nil {
		return "", "", ErrSchemaInvalid
	}
	schema := strings.ToLower(strings.TrimSpace(*t.Schema))
	if !ReIdent.MatchString(schema) {
		return "", "", ErrSchemaInvalid
	}
	return schema, "domain", nil
}
