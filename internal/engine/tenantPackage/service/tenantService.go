package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"skyrix/internal/engine"
	"skyrix/internal/engine/tenantPackage/entity"
	"skyrix/internal/engine/tenantPackage/repository"
	"skyrix/internal/logger"
	"strings"
	"sync"
	"time"
)

type TenantService struct {
	Log         logger.Interface
	Repo        *repository.TenantRepository
	Cache       engine.Cache
	ttl         time.Duration
	KeyPrefix   string
	mu          sync.RWMutex
	byNamespace map[string]*entity.Tenant
	byDomain    map[string]*entity.Tenant
}

type CacheOpts struct {
	TTL       time.Duration
	KeyPrefix string // needed to form keys
}

func NewTenantService(
	log logger.Interface,
	repo *repository.TenantRepository,
	cache engine.Cache,
	opts CacheOpts,
) *TenantService {
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 3 * time.Minute
	}

	prefix := strings.TrimSuffix(strings.TrimSpace(opts.KeyPrefix), ":")
	if prefix == "" {
		prefix = "skyrix-delivery"
	}

	return &TenantService{
		Log:         log,
		Repo:        repo,
		Cache:       cache,
		ttl:         ttl,
		KeyPrefix:   prefix,
		byNamespace: make(map[string]*entity.Tenant),
		byDomain:    make(map[string]*entity.Tenant),
	}
}

var ErrNotFound = errors.New("tenant not found")

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (s *TenantService) redisKeyNamespace(namespace string) string {
	return s.KeyPrefix + ":tenant:namespace:" + norm(namespace)
}
func (s *TenantService) redisKeyDomain(d string) string {
	return s.KeyPrefix + ":tenant:domain:" + norm(d)
}

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

func encodeTenant(t *entity.Tenant) ([]byte, error) {
	return json.Marshal(t)
}

func decodeTenant(b []byte) (*entity.Tenant, error) {
	if len(b) == 0 {
		return nil, nil
	}
	var t entity.Tenant
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	return &t, nil
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

// updateL2Cache populates the Redis cache with the given tenant.
func (s *TenantService) updateL2Cache(ctx context.Context, t *entity.Tenant) {
	if s.Cache == nil || t == nil {
		return
	}

	data, err := encodeTenant(t)
	if err != nil {
		s.Log.Error("failed to encode tenant for cache", "namespace", s.nsVal(t), "error", err)
		return
	}

	if ns := s.nsVal(t); ns != "" {
		if err := s.Cache.Set(ctx, s.redisKeyNamespace(ns), data, s.ttl); err != nil {
			s.Log.Warn("failed to set tenant in cache by namespace", "namespace", ns, "error", err)
		}
	}
	if d := s.domainVal(t); d != "" {
		if err := s.Cache.Set(ctx, s.redisKeyDomain(d), data, s.ttl); err != nil {
			s.Log.Warn("failed to set tenant in cache by domain", "domain", d, "error", err)
		}
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

	// Redis
	if s.Cache != nil {
		key := s.redisKeyNamespace(namespace)
		if b, ok, err := s.Cache.Get(ctx, key); err == nil && ok {
			if t, err := decodeTenant(b); err == nil && s.isActive(t) && s.schemaVal(t) != "" {
				s.updateL1Cache(t)
				return t, nil
			} else if err != nil {
				s.Log.Warn("failed to decode tenant from cache", "key", key, "error", err)
			}
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

	// Redis
	if s.Cache != nil {
		key := s.redisKeyDomain(domain)
		if b, ok, err := s.Cache.Get(ctx, key); err == nil && ok {
			if t, err := decodeTenant(b); err == nil && s.isActive(t) && s.schemaVal(t) != "" {
				s.updateL1Cache(t)
				return t, nil
			} else if err != nil {
				s.Log.Warn("failed to decode tenant from cache", "key", key, "error", err)
			}
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
