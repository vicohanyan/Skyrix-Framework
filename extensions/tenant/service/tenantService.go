package service

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"gitlab.com/skyrix-lib/framework/extensions/tenant/entity"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/repository"
)

// Cache is the shared (L2) tenant cache, implemented by cache.RedisCache.
// A miss returns ok == false and no error.
type Cache interface {
	ByNamespace(ctx context.Context, namespace string) (t *entity.Tenant, ok bool, err error)
	ByDomain(ctx context.Context, domain string) (t *entity.Tenant, ok bool, err error)
	Put(ctx context.Context, t *entity.Tenant) error
}

type TenantService struct {
	Log         logger.Interface
	Repo        *repository.TenantRepository
	Cache       Cache
	mu          sync.RWMutex
	byNamespace map[string]*entity.Tenant
	byDomain    map[string]*entity.Tenant
}

// NewTenantService returns the tenant service. cache may be nil to use only
// the in-memory cache and the repository.
func NewTenantService(
	log logger.Interface,
	repo *repository.TenantRepository,
	cache Cache,
) *TenantService {
	return &TenantService{
		Log:         log,
		Repo:        repo,
		Cache:       cache,
		byNamespace: make(map[string]*entity.Tenant),
		byDomain:    make(map[string]*entity.Tenant),
	}
}

var ErrNotFound = errors.New("tenant not found")

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (s *TenantService) isActive(t *entity.Tenant) bool {
	return t != nil && t.IsActive
}

func (s *TenantService) schemaVal(t *entity.Tenant) string {
	if t == nil || t.Schema == nil {
		return ""
	}
	return norm(*t.Schema)
}

func (s *TenantService) domainVal(t *entity.Tenant) string {
	if t == nil || t.Domain == nil {
		return ""
	}
	return norm(*t.Domain)
}

func (s *TenantService) nsVal(t *entity.Tenant) string {
	if t == nil {
		return ""
	}
	return norm(t.Namespace)
}

// updateL1Cache populates the in-memory cache with the given tenant.
func (s *TenantService) updateL1Cache(t *entity.Tenant) {
	if t == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if ns := s.nsVal(t); ns != "" {
		s.byNamespace[ns] = t
	}
	if d := s.domainVal(t); d != "" {
		s.byDomain[d] = t
	}
}

// updateL2Cache populates the shared cache with the given tenant.
func (s *TenantService) updateL2Cache(ctx context.Context, t *entity.Tenant) {
	if s.Cache == nil || t == nil {
		return
	}
	if err := s.Cache.Put(ctx, t); err != nil {
		s.Log.Warn("failed to set tenant in cache", "namespace", s.nsVal(t), "domain", s.domainVal(t), "error", err)
	}
}

func (s *TenantService) GetByNamespace(ctx context.Context, namespace string) (*entity.Tenant, error) {
	namespace = norm(namespace)
	if namespace == "" {
		return nil, ErrNotFound
	}

	// L1
	s.mu.RLock()
	if t, ok := s.byNamespace[namespace]; ok && s.isActive(t) && s.schemaVal(t) != "" {
		s.mu.RUnlock()
		return t, nil
	}
	s.mu.RUnlock()

	// L2 (shared cache)
	if s.Cache != nil {
		t, ok, err := s.Cache.ByNamespace(ctx, namespace)
		if err != nil {
			s.Log.Warn("failed to read tenant from cache", "namespace", namespace, "error", err)
		} else if ok && s.isActive(t) && s.schemaVal(t) != "" {
			s.updateL1Cache(t)
			return t, nil
		}
	}

	// DB
	t, err := s.Repo.GetByNamespace(ctx, namespace)
	if err != nil {
		return nil, ErrNotFound
	}
	if !s.isActive(t) || s.schemaVal(t) == "" {
		return nil, ErrNotFound
	}

	// store caches
	s.updateL1Cache(t)
	s.updateL2Cache(ctx, t)

	return t, nil
}

func (s *TenantService) GetByDomain(ctx context.Context, domain string) (*entity.Tenant, error) {
	domain = norm(domain)
	if domain == "" {
		return nil, ErrNotFound
	}

	// L1
	s.mu.RLock()
	if t, ok := s.byDomain[domain]; ok && s.isActive(t) && s.schemaVal(t) != "" {
		s.mu.RUnlock()
		return t, nil
	}
	s.mu.RUnlock()

	// L2 (shared cache)
	if s.Cache != nil {
		t, ok, err := s.Cache.ByDomain(ctx, domain)
		if err != nil {
			s.Log.Warn("failed to read tenant from cache", "domain", domain, "error", err)
		} else if ok && s.isActive(t) && s.schemaVal(t) != "" {
			s.updateL1Cache(t)
			return t, nil
		}
	}

	// DB
	t, err := s.Repo.GetByDomain(ctx, domain)
	if err != nil {
		return nil, ErrNotFound
	}
	if !s.isActive(t) || s.schemaVal(t) == "" {
		return nil, ErrNotFound
	}

	// store caches
	s.updateL1Cache(t)
	s.updateL2Cache(ctx, t)

	return t, nil
}

func (s *TenantService) ListDomains(ctx context.Context) ([]string, error) {
	if s.Repo == nil {
		return nil, fmt.Errorf("repo is nil")
	}
	return s.Repo.ListDomains(ctx)
}
