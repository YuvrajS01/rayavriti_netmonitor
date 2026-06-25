package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
)

type scopeContextKey int

const scopeKey scopeContextKey = 0

type ScopeContext struct {
	UserID   int64
	Role     string
	Scopes   []UserScope
	IsScoped bool
}

type UserScope struct {
	Type  string
	Value string
}

func RequireScopeContext(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"success":false,"error":"not authenticated"}`, http.StatusUnauthorized)
				return
			}

			sc := &ScopeContext{
				UserID: claims.UserID,
				Role:   claims.Role,
			}

			if claims.Role == "super_admin" || claims.Role == "admin" {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), scopeKey, sc)))
				return
			}

			sc.IsScoped = true

			rows, err := pool.Query(r.Context(),
				"SELECT scope_type, scope_value FROM user_scopes WHERE user_id = $1",
				claims.UserID)
			if err != nil {
				slog.Error("scope: failed to load scopes",
					"user_id", claims.UserID,
					"error", err,
				)
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), scopeKey, sc)))
				return
			}
			defer rows.Close()

			for rows.Next() {
				var us UserScope
				if err := rows.Scan(&us.Type, &us.Value); err != nil {
					slog.Error("scope: failed to scan scope",
						"user_id", claims.UserID,
						"error", err,
					)
					continue
				}
				sc.Scopes = append(sc.Scopes, us)
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), scopeKey, sc)))
		})
	}
}

func GetScopeContext(r *http.Request) *ScopeContext {
	sc, _ := r.Context().Value(scopeKey).(*ScopeContext)
	return sc
}

func FilterDeviceQuery(sc *ScopeContext, baseQuery string) string {
	if sc == nil || !sc.IsScoped || len(sc.Scopes) == 0 {
		return baseQuery
	}

	locationIDs := make([]string, 0)
	subnetCIDRs := make([]string, 0)

	for _, s := range sc.Scopes {
		switch s.Type {
		case "location":
			locationIDs = append(locationIDs, s.Value)
		case "subnet":
			subnetCIDRs = append(subnetCIDRs, s.Value)
		}
	}

	if len(locationIDs) == 0 && len(subnetCIDRs) == 0 {
		return baseQuery + " AND 1=0"
	}

	conditions := make([]string, 0)

	if len(locationIDs) > 0 {
		conditions = append(conditions,
			fmt.Sprintf("d.location_id IN (%s)", strings.Join(locationIDs, ",")))
	}

	if len(subnetCIDRs) > 0 {
		for _, cidr := range subnetCIDRs {
			conditions = append(conditions,
				fmt.Sprintf("d.ip_address >>= '%s'", strings.ReplaceAll(cidr, "'", "''")))
		}
	}

	return fmt.Sprintf("%s AND (%s)", baseQuery, strings.Join(conditions, " OR "))
}

func FilterAlertQuery(sc *ScopeContext, baseQuery string) string {
	if sc == nil || !sc.IsScoped || len(sc.Scopes) == 0 {
		return baseQuery
	}

	locationIDs := make([]string, 0)
	subnetCIDRs := make([]string, 0)

	for _, s := range sc.Scopes {
		switch s.Type {
		case "location":
			locationIDs = append(locationIDs, s.Value)
		case "subnet":
			subnetCIDRs = append(subnetCIDRs, s.Value)
		}
	}

	if len(locationIDs) == 0 && len(subnetCIDRs) == 0 {
		return baseQuery + " AND 1=0"
	}

	conditions := make([]string, 0)

	if len(locationIDs) > 0 {
		conditions = append(conditions,
			fmt.Sprintf("a.location_id IN (%s)", strings.Join(locationIDs, ",")))
	}

	if len(subnetCIDRs) > 0 {
		for _, cidr := range subnetCIDRs {
			conditions = append(conditions,
				fmt.Sprintf("a.device_id IN (SELECT id FROM devices WHERE ip_address >>= '%s')",
					strings.ReplaceAll(cidr, "'", "''")))
		}
	}

	return fmt.Sprintf("%s AND (%s)", baseQuery, strings.Join(conditions, " OR "))
}
