package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
	"github.com/rayavriti/netmonitor-backend/internal/httputil"
	"github.com/rayavriti/netmonitor-backend/internal/models"
	"github.com/rayavriti/netmonitor-backend/internal/rbac"
)

// Scope guards enforce location/subnet scopes on routes that address a
// single resource by ID. Out-of-scope resources answer 404, exactly like
// missing ones, so scoped users can't probe which IDs exist (S7).

func isScoped(r *http.Request) bool {
	sc := rbac.GetScopeContext(r)
	return sc != nil && sc.IsScoped
}

func notFound(w http.ResponseWriter) {
	httputil.SendError(w, http.StatusNotFound, "not found")
}

func urlID(r *http.Request, param string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, param), 10, 64)
	return id, err == nil
}

// DeviceScopeGuard admits a scoped user only if the device named by the URL
// parameter is within their scopes.
func DeviceScopeGuard(db database.Database, param string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isScoped(r) {
				next.ServeHTTP(w, r)
				return
			}
			id, ok := urlID(r, param)
			if !ok {
				httputil.SendError(w, http.StatusBadRequest, "invalid id")
				return
			}
			device, err := db.GetDevice(r.Context(), id)
			if err != nil || device == nil || !canAccessDevice(r, device) {
				notFound(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// AlertScopeGuard admits a scoped user only if the alert's device is within
// their scopes. Alerts without a device are denied to scoped users.
func AlertScopeGuard(db database.Database, param string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isScoped(r) {
				next.ServeHTTP(w, r)
				return
			}
			id, ok := urlID(r, param)
			if !ok {
				httputil.SendError(w, http.StatusBadRequest, "invalid id")
				return
			}
			alert, err := db.GetAlert(r.Context(), id)
			if err != nil || alert == nil || alert.DeviceID == 0 {
				notFound(w)
				return
			}
			device, err := db.GetDevice(r.Context(), alert.DeviceID)
			if err != nil || !canAccessAlert(r, alert, device) {
				notFound(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LocationScopeGuard admits a scoped user only to locations named in one of
// their location scopes.
func LocationScopeGuard(param string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isScoped(r) {
				next.ServeHTTP(w, r)
				return
			}
			want := chi.URLParam(r, param)
			for _, s := range rbac.GetScopeContext(r).Scopes {
				if s.Type == "location" && s.Value == want {
					next.ServeHTTP(w, r)
					return
				}
			}
			notFound(w)
		})
	}
}

// RequireUnscoped restricts network-wide data that has no per-device owner
// (flows, packet capture, raw metric queries, reports, root-cause analysis)
// to users without scopes.
func RequireUnscoped(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isScoped(r) {
			httputil.SendError(w, http.StatusForbidden, "not available to location-scoped users")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// scopedDeviceIDs returns the set of device IDs the request may see, or nil
// when the user is unscoped (sees everything).
func scopedDeviceIDs(ctx context.Context, r *http.Request, db database.Database) (map[int64]bool, error) {
	sf := scopeFilterFromContext(r)
	if sf == nil {
		return nil, nil
	}
	devices, _, err := db.GetDevicesFiltered(ctx, database.DeviceFilter{Scope: sf, Limit: 100000})
	if err != nil {
		return nil, err
	}
	ids := make(map[int64]bool, len(devices))
	for _, d := range devices {
		ids[d.ID] = true
	}
	return ids, nil
}

// canPlaceInLocation reports whether the request may put a device in the
// given location. Scoped users may only use locations in their scopes (and
// may not leave a device unplaced, which would take it out of their view).
func canPlaceInLocation(r *http.Request, locationID *int64) bool {
	if !isScoped(r) {
		return true
	}
	if locationID == nil {
		return false
	}
	want := strconv.FormatInt(*locationID, 10)
	for _, s := range rbac.GetScopeContext(r).Scopes {
		if s.Type == "location" && s.Value == want {
			return true
		}
	}
	return false
}

// filterScoresByScope drops health scores for devices outside the caller's
// scopes. Unscoped callers get the input back unchanged.
func filterScoresByScope(r *http.Request, db database.Database, scores []models.DeviceHealthScoreRow) ([]models.DeviceHealthScoreRow, error) {
	allowed, err := scopedDeviceIDs(r.Context(), r, db)
	if err != nil || allowed == nil {
		return scores, err
	}
	kept := make([]models.DeviceHealthScoreRow, 0, len(scores))
	for _, s := range scores {
		if allowed[s.DeviceID] {
			kept = append(kept, s)
		}
	}
	return kept, nil
}

// canWriteDevices reports whether the caller may see device credentials.
func canWriteDevices(r *http.Request) bool {
	claims := auth.GetClaims(r.Context())
	if claims == nil {
		return false
	}
	return rbac.IsUnscopedRole(claims.Role) || rbac.HasPermission(claims.Permissions, models.PermDevicesWrite)
}

// redactDeviceSecrets blanks SNMP credentials for callers who can only read
// devices: a community string is a credential for the device itself (S10).
// Writers keep the community but never receive SNMPv3 passphrases.
func redactDeviceSecrets(r *http.Request, devices []models.Device) {
	for i := range devices {
		redactDevice(r, &devices[i])
	}
}

func redactDevice(r *http.Request, d *models.Device) {
	if d == nil {
		return
	}
	if !canWriteDevices(r) {
		d.SNMPCommunity = ""
		d.SNMPv3 = nil
		return
	}
	if d.SNMPv3 != nil {
		// Copy so the caller's (possibly shared) config keeps its secrets.
		v3 := *d.SNMPv3
		if v3.AuthPassphrase != "" {
			v3.AuthPassphrase = secretPlaceholder
		}
		if v3.PrivPassphrase != "" {
			v3.PrivPassphrase = secretPlaceholder
		}
		d.SNMPv3 = &v3
	}
}

// mergeSNMPv3 resolves an incoming SNMPv3 block against the stored one: a
// passphrase sent back as the mask placeholder keeps the stored value.
func mergeSNMPv3(incoming, stored *models.SNMPv3Config) *models.SNMPv3Config {
	if incoming == nil {
		return nil
	}
	out := *incoming
	if out.AuthPassphrase == secretPlaceholder {
		out.AuthPassphrase = ""
		if stored != nil {
			out.AuthPassphrase = stored.AuthPassphrase
		}
	}
	if out.PrivPassphrase == secretPlaceholder {
		out.PrivPassphrase = ""
		if stored != nil {
			out.PrivPassphrase = stored.PrivPassphrase
		}
	}
	return &out
}

// validateSNMPSettings checks the SNMP version and, for v3, its credentials.
func validateSNMPSettings(d *models.Device) error {
	switch d.SNMPVersion {
	case "", "1", "2c":
		return nil
	case "3":
		if d.SNMPv3 == nil {
			return errors.New("snmpV3 credentials are required for SNMP version 3")
		}
		return d.SNMPv3.Validate()
	default:
		return errors.New("snmpVersion must be one of 1, 2c, 3")
	}
}
