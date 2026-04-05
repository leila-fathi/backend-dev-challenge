package auth

import (
	"errors"
	"net/http"

	"hiring-challenge-backend/api/internal/requestctx"
)

func ContextMiddleware(tokenManager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ExtractBearerToken(r.Header.Get("Authorization"))

			if token != "" {
				claims, err := tokenManager.ParseAccessToken(token)
				if err != nil {
					http.Error(w, "invalid access token", http.StatusUnauthorized)
					return
				}
				ctx := requestctx.WithTokenClaims(r.Context(), claims)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireUserClaims(r *http.Request) (*TokenClaims, error) {
	rawClaims, ok := requestctx.TokenClaimsFromContext(r.Context())
	if !ok || rawClaims == nil {
		return nil, errors.New("access denied")
	}
	claims, ok := rawClaims.(*TokenClaims)
	if !ok || claims == nil {
		return nil, errors.New("access denied")
	}
	return claims, nil
}
