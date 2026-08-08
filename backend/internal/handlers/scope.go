package handlers

import (
	"net/http"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/rbac"
)

// scopeFilterFromContext returns the tenant scope for the current request, or
// nil for admins/unauthenticated access so no filtering is applied.
func scopeFilterFromContext(r *http.Request) *database.ScopeFilter {
	return rbac.GetScopeContext(r).DatabaseScopeFilter()
}
