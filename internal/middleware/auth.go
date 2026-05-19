package middleware

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
)

const AuthenticatedKey = "authenticated"

func AuthMiddleware(sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authenticated := sessionManager.GetBool(r.Context(), AuthenticatedKey)
			if !authenticated {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func SetAuthenticated(ctx context.Context, sessionManager *scs.SessionManager) {
	sessionManager.Put(ctx, AuthenticatedKey, true)
}

func ClearAuthenticated(ctx context.Context, sessionManager *scs.SessionManager) {
	sessionManager.Remove(ctx, AuthenticatedKey)
}
