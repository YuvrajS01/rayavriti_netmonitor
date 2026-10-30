package rbac

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rayavriti/netmonitor-backend/internal/auth"
	"github.com/rayavriti/netmonitor-backend/internal/database"
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

			if IsUnscopedRole(claims.Role) {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), scopeKey, sc)))
				return
			}

			sc.IsScoped = true

			scopes, err := LoadScopes(r.Context(), pool, claims.UserID)
			if err != nil {
				// Fail closed: a scoped user with unknown scopes sees nothing.
				slog.Error("scope: failed to load scopes", "user_id", claims.UserID, "error", err)
			}
			sc.Scopes = scopes

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), scopeKey, sc)))
		})
	}
}

// IsUnscopedRole reports whether a role sees all devices regardless of
// user_scopes rows.
func IsUnscopedRole(role string) bool {
	return role == "super_admin" || role == "admin"
}

// LoadScopes returns the scope rows assigned to a user.
func LoadScopes(ctx context.Context, pool *pgxpool.Pool, userID int64) ([]UserScope, error) {
	rows, err := pool.Query(ctx, "SELECT scope_type, scope_value FROM user_scopes WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scopes []UserScope
	for rows.Next() {
		var us UserScope
		if err := rows.Scan(&us.Type, &us.Value); err != nil {
			return nil, err
		}
		scopes = append(scopes, us)
	}
	return scopes, rows.Err()
}

func GetScopeContext(r *http.Request) *ScopeContext {
	sc, _ := r.Context().Value(scopeKey).(*ScopeContext)
	return sc
}

// WithScopeContext returns a shallow copy of the request carrying the given
// scope context. Useful for tests and for wiring scopes into sub-handlers.
func WithScopeContext(r *http.Request, sc *ScopeContext) *http.Request {
	if sc == nil {
		return r
	}
	return r.WithContext(context.WithValue(r.Context(), scopeKey, sc))
}

// DatabaseScopeFilter converts a request scope context into a
// database.ScopeFilter so data-fetching handlers can narrow their SQL queries.
func (sc *ScopeContext) DatabaseScopeFilter() *database.ScopeFilter {
	if sc == nil || !sc.IsScoped {
		return nil
	}
	sf := &database.ScopeFilter{}
	for _, s := range sc.Scopes {
		switch s.Type {
		case "location":
			sf.LocationIDs = append(sf.LocationIDs, s.Value)
		case "subnet":
			sf.SubnetCIDRs = append(sf.SubnetCIDRs, s.Value)
		}
	}
	return sf
}
