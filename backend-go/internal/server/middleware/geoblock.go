package middleware

import (
	"net/http"
	"strings"
)

const geoblockHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>451 Unavailable For Legal Reasons</title>
</head>
<body>
  <h1>Not available in your region</h1>
  <p>This service is not available in your region for legal reasons.</p>
</body>
</html>
`

// blockedRegionCodes contains all 27 EU member states, 3 additional EEA states (IS, LI, NO),
// and the UK (both ISO 3166-1 alpha-2 "GB" and colloquial "UK") per ADR-008.
var blockedRegionCodes = map[string]struct{}{
	"AT": {}, "BE": {}, "BG": {}, "HR": {}, "CY": {},
	"CZ": {}, "DK": {}, "EE": {}, "FI": {}, "FR": {},
	"DE": {}, "GR": {}, "HU": {}, "IE": {}, "IT": {},
	"LV": {}, "LT": {}, "LU": {}, "MT": {}, "NL": {},
	"PL": {}, "PT": {}, "RO": {}, "SK": {}, "SI": {},
	"ES": {}, "SE": {},
	// EEA
	"IS": {}, "LI": {}, "NO": {},
	// UK
	"GB": {}, "UK": {},
}

// IsBlockedRegion returns true if countryCode (case-insensitive) is in the EU/EEA/UK geoblock list.
func IsBlockedRegion(countryCode string) bool {
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if code == "" {
		return false
	}
	_, blocked := blockedRegionCodes[code]
	return blocked
}

// Geoblock enforces EU/EEA/UK legal region blocking on anonymous reader and correction endpoints
// per ADR-008 and SECURITY_ARCHITECTURE.md T8.
//
// Trust model:
//   - Production MUST run behind Cloudflare (or an equivalent edge proxy) with origin network
//     isolation so only trusted edge IPs can reach the origin and set CF-IPCountry.
//   - If CF-IPCountry is absent (local development / direct sandbox hit), the request is allowed.
//   - If CF-IPCountry matches a blocked region, HTTP 451 Unavailable For Legal Reasons is returned
//     with a static HTML notice and zero application data.
//   - MUST run BEFORE rate-limit middleware so blocked visitors do not consume rate-limit budgets.
func Geoblock(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		country := r.Header.Get("CF-IPCountry")
		if IsBlockedRegion(country) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusUnavailableForLegalReasons)
			_, _ = w.Write([]byte(geoblockHTML))
			return
		}
		next.ServeHTTP(w, r)
	})
}
