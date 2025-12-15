package schemaResolver

import (
	"errors"
	"net/http"
	"skyrix/internal/engine/tenantPackage/service"
)

type Resolver interface {
	ResolveSchema(req *http.Request) (schema string, resolvedBy string, err error)
}

type SchemaResolver struct {
	order []string
	reg   map[string]Resolver
}

func NewSchemaResolver(svc *service.TenantService, header string, order []string) *SchemaResolver {
	reg := map[string]Resolver{
		"header": NewHeaderResolver(svc, header),
		"domain": NewDomainResolver(svc),
	}
	if len(order) == 0 {
		order = []string{"header", "domain"}
	}
	return &SchemaResolver{order: order, reg: reg}
}

func (s *SchemaResolver) ResolveSchema(req *http.Request) (string, string, error) {
	var last error
	for _, name := range s.order {
		r := s.reg[name]
		if r == nil {
			continue
		}
		schema, by, err := r.ResolveSchema(req)
		if err == nil {
			return schema, by, nil
		}
		if errorsIsSoft(err) {
			last = err
			continue
		}
		return "", "", err
	}
	if last == nil {
		last = ErrTenantNotFound
	}
	return "", "", last
}

func errorsIsSoft(err error) bool {
	return errors.Is(err, ErrTenantHeaderMissing) ||
		errors.Is(err, ErrHostEmpty) ||
		errors.Is(err, ErrTenantNotFound) ||
		errors.Is(err, ErrTenantNotFoundHost)
}
