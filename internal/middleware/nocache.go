package middleware

import "github.com/gin-gonic/gin"

// NoStoreAPI marks API responses as private and non-cacheable so browsers,
// shared HTTP caches, and Cloudflare edge cannot serve stale authenticated
// data.
//
// Without this header browsers cache the response for any URL whose method
// is GET (their default heuristic), which makes "list" endpoints return
// stale results after the user creates / uploads / deletes through the same
// SPA session. End-to-end Playwright traces against the demo box showed
// GET .../knowledge-bases/:id/documents returning total=1 when the DB had
// two rows because the first response had been pinned in the browser's
// disk cache (same cf-ray on second hit).
//
// The header set mirrors what most JSON APIs recommend:
//
//	Cache-Control: no-store, max-age=0, private, must-revalidate
//	Pragma: no-cache
//	Expires: 0
//
// HTTP/1.1 clients honour Cache-Control; the other two are belt-and-braces
// for older intermediaries that fall back to legacy directives.
func NoStoreAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Cache-Control", "no-store, max-age=0, private, must-revalidate")
		h.Set("Pragma", "no-cache")
		h.Set("Expires", "0")
		c.Next()
	}
}
