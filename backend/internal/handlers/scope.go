package handlers

import (
	"fmt"
	"net/http"
	"net/netip"

	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/rbac"
)

// scopeFilterFromContext returns the tenant scope for the current request, or
// nil for admins/unauthenticated access so no filtering is applied.
func scopeFilterFromContext(r *http.Request) *database.ScopeFilter {
	return rbac.GetScopeContext(r).DatabaseScopeFilter()
}

// canAccessDevice checks whether the current request's scope permits access
// to the given device. Returns true for unscoped (admin) users. For scoped
// users, the device must match at least one location or subnet scope.
func canAccessDevice(r *http.Request, device *models.Device) bool {
	sc := rbac.GetScopeContext(r)
	if sc == nil || !sc.IsScoped {
		return true
	}
	return deviceMatchesScope(device, sc)
}

// canAccessAlert checks whether the current request's scope permits access to
// the given alert. Returns true for unscoped (admin) users. For scoped users,
// the alert's device must match at least one location or subnet scope. If the
// alert carries no device/location info, we deny by default.
func canAccessAlert(r *http.Request, alert *models.Alert, device *models.Device) bool {
	sc := rbac.GetScopeContext(r)
	if sc == nil || !sc.IsScoped {
		return true
	}
	if device != nil {
		return deviceMatchesScope(device, sc)
	}
	// If we can't load the device, deny by default for scoped users.
	return false
}

// deviceMatchesScope returns true if the device falls within any of the user's
// scopes (location or subnet).
func deviceMatchesScope(device *models.Device, sc *rbac.ScopeContext) bool {
	for _, s := range sc.Scopes {
		switch s.Type {
		case "location":
			if device.LocationID != nil && fmt.Sprintf("%d", *device.LocationID) == s.Value {
				return true
			}
		case "subnet":
			if device.IPAddress != "" {
				// Check if the device IP falls within the subnet CIDR.
				if ip, err := netip.ParseAddr(device.IPAddress); err == nil && ip.IsValid() {
					if prefix, err := netip.ParsePrefix(s.Value); err == nil {
						if prefix.Contains(ip) {
							return true
						}
					}
				}
			}
		}
	}
	return false
}
