package middleware

import (
	"context"
	"net/http"
	"net/url"
	"skyrix/internal/engine/tenantPackage/repository"
	"strings"
	"sync"
	"time"

	"skyrix/internal/logger"
)

type AllowedDomain struct {
	Domain   string
	Wildcard bool
}

type CorsMiddleware struct {
	TenantRepository *repository.TenantRepository
	Logger           logger.Interface

	MU          sync.RWMutex
	Domains     []AllowedDomain
	LastFetched time.Time
	CacheTTL    time.Duration
}

func NewCorsMiddleware(tenantRepository *repository.TenantRepository, logger logger.Interface) *CorsMiddleware {
	return &CorsMiddleware{
		TenantRepository: tenantRepository,
		Logger:           logger,
		CacheTTL:         3 * time.Minute, // configurable
	}
}

func parseAllowedDomains(domainList []string) []AllowedDomain {
	var allowed []AllowedDomain
	for _, d := range domainList {
		d = strings.ToLower(strings.TrimSpace(d))
		if strings.HasPrefix(d, "*.") {
			allowed = append(allowed, AllowedDomain{
				Domain:   strings.TrimPrefix(d, "*."),
				Wildcard: true,
			})
		} else {
			allowed = append(allowed, AllowedDomain{
				Domain:   d,
				Wildcard: false,
			})
		}
	}
	return allowed
}

func (m *CorsMiddleware) getDomains(ctx context.Context) []AllowedDomain {
	m.MU.RLock()
	if time.Since(m.LastFetched) < m.CacheTTL {
		domains := m.Domains
		m.MU.RUnlock()
		return domains
	}
	m.MU.RUnlock()

	m.MU.Lock()
	defer m.MU.Unlock()
	// Double-check after acquiring the lock
	if time.Since(m.LastFetched) < m.CacheTTL {
		return m.Domains
	}
	rawDomains, err := m.TenantRepository.ListDomains(ctx)
	if err != nil {
		m.Logger.Warn("CORS: Failed to update domains", "error", err)
		// Return stale data on error
		return m.Domains
	}
	m.Domains = parseAllowedDomains(rawDomains)
	m.LastFetched = time.Now()
	return m.Domains
}

func isAllowedOrigin(originHost string, allowed []AllowedDomain) bool {
	for _, dom := range allowed {
		if dom.Wildcard {
			if originHost == dom.Domain || strings.HasSuffix(originHost, "."+dom.Domain) {
				return true
			}
		} else {
			if originHost == dom.Domain {
				return true
			}
		}
	}
	return false
}

func (m *CorsMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")

		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		u, err := url.Parse(origin)
		if err != nil {
			m.Logger.Warn("CORS: Invalid Origin", "origin", origin, "error", err)
			next.ServeHTTP(w, r)
			return
		}
		originHost := strings.ToLower(u.Hostname())
		allowed := isAllowedOrigin(originHost, m.getDomains(r.Context()))

		if allowed {
			reqHdr := r.Header.Get("Access-Control-Request-Headers")

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if reqHdr != "" {
				w.Header().Set("Access-Control-Allow-Headers", reqHdr)
			} else {
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Expose-Headers", "X-Tenant-Resolved-By")
			w.Header().Set("Access-Control-Max-Age", "180")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
		} else {
			if origin != "" {
				m.Logger.Warn("CORS: Blocked origin", "origin", origin)
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
