package schemaResolver

import (
	"net"
	"net/http"
	"regexp"
	"strings"
)

var ReIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

const DefaultTenantHeader = "X-Tenant"

func hostFromRequest(req *http.Request) string {
	h := strings.TrimSpace(req.Header.Get("X-Forwarded-Host"))
	if h == "" {
		h = req.Host
	}
	if strings.Contains(h, ":") {
		if host, _, err := net.SplitHostPort(h); err == nil {
			return strings.ToLower(host)
		}
	}
	return strings.ToLower(h)
}
