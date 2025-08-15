package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	"time"
)

func cacheKeyFromRequest(r *http.Request) string {
	// Key: METHOD|PATH|sorted(query)
	// Host/scheme are intentionally excluded to keep keys stable across proxy ↔ upstream phases.
	b := strings.Builder{}
	b.WriteString(r.Method)
	b.WriteString("|")
	b.WriteString(r.URL.Path)
	b.WriteString("|")
	// Sorted query to normalize key
	keys := make([]string, 0, len(r.URL.Query()))
	for k := range r.URL.Query() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vals := r.URL.Query()[k]
		sort.Strings(vals)
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(strings.Join(vals, ","))
		b.WriteString("&")
	}
	raw := b.String()
	sum := sha256.Sum256([]byte(raw))
	baseKey := hex.EncodeToString(sum[:])

	// Include a conservative Vary subset from request headers to avoid mismatched reps
	// Pragmatic approach until per-response Vary handling is added.
	varyHeaders := []string{"Accept", "Accept-Encoding", "Accept-Language"}
	vb := strings.Builder{}
	for _, hk := range varyHeaders {
		if v := r.Header.Get(hk); v != "" {
			vb.WriteString(strings.ToLower(hk))
			vb.WriteString(":")
			vb.WriteString(strings.TrimSpace(v))
			vb.WriteString("|")
		}
	}
	vraw := baseKey + vb.String()
	vsum := sha256.Sum256([]byte(vraw))
	varyKey := hex.EncodeToString(vsum[:])

	final := strings.Builder{}
	final.WriteString(varyKey)

	// For methods with body, include X-Body-Hash when present (computed in proxy)
	if r.Header.Get("X-Body-Hash") != "" {
		final.WriteString("|body=")
		final.WriteString(r.Header.Get("X-Body-Hash"))
	}

	// If client explicitly opts into shared caching with a TTL (public + max-age/s-maxage),
	// include the requested TTL in the key so different TTLs do not collide with older entries.
	// Only apply this for methods that can carry a body (POST/PUT/PATCH) to avoid splitting
	// GET/HEAD cache keys unnecessarily when origin already provides TTLs.
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		cc := parseCacheControl(r.Header.Get("Cache-Control"))
		if cc.publicCache && (cc.sMaxAge > 0 || cc.maxAge > 0) {
			final.WriteString("|ttl=")
			if cc.sMaxAge > 0 {
				final.WriteString("smax=")
				final.WriteString(strconv.Itoa(cc.sMaxAge))
			} else {
				final.WriteString("max=")
				final.WriteString(strconv.Itoa(cc.maxAge))
			}
		}
	}

	return final.String()
}

func isResponseCacheable(res *http.Response) bool {
	if res == nil {
		return false
	}
	cc := parseCacheControl(res.Header.Get("Cache-Control"))
	if cc.noStore || cc.privateCache {
		return false
	}
	// Cacheable status codes (basic set)
	switch res.StatusCode {
	case 200, 203, 301, 308, 404, 410:
		// ok
	default:
		return false
	}
	// If Authorization was present on request, require explicit shared cache directives
	if res.Request != nil {
		if res.Request.Header.Get("Authorization") != "" {
			if !(cc.publicCache || cc.sMaxAge > 0) {
				return false
			}
		}
	}
	// Don't cache SSE
	if strings.Contains(res.Header.Get("Content-Type"), "text/event-stream") {
		return false
	}
	if res.Header.Get("Vary") == "*" {
		return false
	}
	return true
}

type cacheControl struct {
	noStore      bool
	noCache      bool
	mustReval    bool
	maxAge       int
	sMaxAge      int
	publicCache  bool
	privateCache bool
}

func parseCacheControl(v string) cacheControl {
	cc := cacheControl{}
	parts := strings.Split(v, ",")
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		switch {
		case p == "no-store":
			cc.noStore = true
		case p == "no-cache":
			cc.noCache = true
		case p == "must-revalidate":
			cc.mustReval = true
		case p == "public":
			cc.publicCache = true
		case p == "private":
			cc.privateCache = true
		case strings.HasPrefix(p, "s-maxage="):
			cc.sMaxAge = atoiSafe(strings.TrimPrefix(p, "s-maxage="))
		case strings.HasPrefix(p, "max-age="):
			cc.maxAge = atoiSafe(strings.TrimPrefix(p, "max-age="))
		}
	}
	return cc
}

func cacheTTLFromHeaders(res *http.Response, defaultTTL time.Duration) time.Duration {
	cc := parseCacheControl(res.Header.Get("Cache-Control"))
	if cc.noStore {
		return 0
	}
	if cc.sMaxAge > 0 {
		return time.Duration(cc.sMaxAge) * time.Second
	}
	if cc.maxAge > 0 {
		return time.Duration(cc.maxAge) * time.Second
	}
	if defaultTTL > 0 && (cc.publicCache || (!cc.privateCache && !cc.noCache)) {
		return defaultTTL
	}
	return 0
}

// requestForcedCacheTTL returns a TTL requested by the client via Cache-Control
// when the client explicitly asks for shared caching (public) and provides a TTL.
// This is primarily used for benchmarking when upstream does not send cache hints.
func requestForcedCacheTTL(req *http.Request) time.Duration {
	if req == nil {
		return 0
	}
	cc := parseCacheControl(req.Header.Get("Cache-Control"))
	if !cc.publicCache {
		return 0
	}
	if cc.sMaxAge > 0 {
		return time.Duration(cc.sMaxAge) * time.Second
	}
	if cc.maxAge > 0 {
		return time.Duration(cc.maxAge) * time.Second
	}
	return 0
}

func atoiSafe(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

func cloneHeadersForCache(h http.Header) http.Header {
	// Drop hop-by-hop headers
	drop := map[string]struct{}{
		"Connection":          {},
		"Keep-Alive":          {},
		"Proxy-Authenticate":  {},
		"Proxy-Authorization": {},
		"TE":                  {},
		"Trailers":            {},
		"Transfer-Encoding":   {},
		"Upgrade":             {},
	}
	out := http.Header{}
	for k, vals := range h {
		if _, ok := drop[k]; ok {
			continue
		}
		for _, v := range vals {
			out.Add(k, v)
		}
	}
	return out
}

// canServeCachedForRequest decides if a cached response is reusable for the given request.
// In particular, for requests with Authorization, only allow reuse when the cached
// response explicitly allows shared caching (public or s-maxage>0).
func canServeCachedForRequest(r *http.Request, cachedHeaders http.Header) bool {
	if r == nil {
		return false
	}
	if r.Header.Get("Authorization") == "" {
		return true
	}
	cc := parseCacheControl(cachedHeaders.Get("Cache-Control"))
	if cc.publicCache || cc.sMaxAge > 0 {
		return true
	}
	return false
}

// conditionalRequestMatches returns true if the client's conditional headers
// (If-None-Match or If-Modified-Since) match the cached response headers.
// RFC semantics simplified for strong validators; good enough for proxy cache use.
func conditionalRequestMatches(r *http.Request, cachedHeaders http.Header) bool {
	if r == nil {
		return false
	}
	// If-None-Match takes precedence over If-Modified-Since
	if inm := strings.TrimSpace(r.Header.Get("If-None-Match")); inm != "" {
		// Canonicalize header name and compare values (support multiple via comma)
		etag := strings.TrimSpace(cachedHeaders.Get(textproto.CanonicalMIMEHeaderKey("ETag")))
		if etag == "" {
			return false
		}
		// Strip surrounding quotes for comparison robustness
		ce := strings.Trim(etag, "\"")
		for _, part := range strings.Split(inm, ",") {
			p := strings.TrimSpace(part)
			p = strings.Trim(p, "\"")
			if p == "*" || p == ce {
				return true
			}
		}
		return false
	}
	if ims := strings.TrimSpace(r.Header.Get("If-Modified-Since")); ims != "" {
		lm := strings.TrimSpace(cachedHeaders.Get(textproto.CanonicalMIMEHeaderKey("Last-Modified")))
		if lm == "" {
			return false
		}
		imsTime, err1 := http.ParseTime(ims)
		lmTime, err2 := http.ParseTime(lm)
		if err1 != nil || err2 != nil {
			return false
		}
		if !lmTime.After(imsTime) {
			return true
		}
	}
	return false
}

// hasClientCacheOptIn returns true if the client request explicitly opts into
// shared caching via Cache-Control (public with max-age or s-maxage > 0).
func hasClientCacheOptIn(r *http.Request) bool {
	if r == nil {
		return false
	}
	cc := parseCacheControl(r.Header.Get("Cache-Control"))
	if !cc.publicCache {
		return false
	}
	if cc.sMaxAge > 0 {
		return true
	}
	if cc.maxAge > 0 {
		return true
	}
	return false
}

// hasClientConditionals reports if the client sent If-None-Match or If-Modified-Since.
func hasClientConditionals(r *http.Request) bool {
	if r == nil {
		return false
	}
	if strings.TrimSpace(r.Header.Get("If-None-Match")) != "" {
		return true
	}
	if strings.TrimSpace(r.Header.Get("If-Modified-Since")) != "" {
		return true
	}
	return false
}

// wantsRevalidation returns true if the client requests origin revalidation
// (e.g., Cache-Control: no-cache or max-age=0).
func wantsRevalidation(r *http.Request) bool {
	if r == nil {
		return false
	}
	ccVal := strings.ToLower(r.Header.Get("Cache-Control"))
	if ccVal == "" {
		return false
	}
	if strings.Contains(ccVal, "no-cache") {
		return true
	}
	if strings.Contains(ccVal, "max-age=0") {
		return true
	}
	return false
}
