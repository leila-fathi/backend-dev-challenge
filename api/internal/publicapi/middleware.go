package publicapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"hiring-challenge-backend/api/internal/session"

	"github.com/google/uuid"
)

type sessionContextKey string

const restSessionKey sessionContextKey = "rest_session"

type SessionContext struct {
	SessionID     uuid.UUID
	UserID        uuid.UUID
	ActiveGroupID uuid.UUID
}

// for storing session info in the request context.
func WithSession(ctx context.Context, s SessionContext) context.Context {
	return context.WithValue(ctx, restSessionKey, s)
}

// for retrieving session info from context
func SessionFromContext(ctx context.Context) (SessionContext, bool) {
	v := ctx.Value(restSessionKey)
	s, ok := v.(SessionContext)
	return s, ok
}

// Enforcing only authenticated users can access protected endpoints, while allowing session info to flow through handlers.
func AuthMiddleware(store *session.Store) func(http.Handler) http.Handler {
	if store == nil {
		// Wiring/config error: fail fast instead of returning runtime 500s for every request.
		panic("publicapi auth middleware requires non-nil session store")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := extractBearerToken(r.Header.Get("Authorization"))
			if raw == "" {
				writeUnauthorized(w, "missing bearer token")
				return
			}

			rec, err := store.GetByRawToken(r.Context(), raw)
			if err != nil {
				if errors.Is(err, session.ErrInvalidSession) {
					writeUnauthorized(w, "invalid session")
					return
				}

				// Internal error path: log for diagnostics, return generic response to caller.
				log.Printf("publicapi auth middleware session lookup failed: method=%s path=%s err=%v", r.Method, r.URL.Path, err)
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{
					Code:    "internal_error",
					Message: "internal server error",
				})
				return
			}

			ctx := WithSession(r.Context(), SessionContext{
				SessionID:     rec.ID,
				UserID:        rec.UserID,
				ActiveGroupID: rec.ActiveGroupID,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(v string) string {
	parts := strings.Fields(strings.TrimSpace(v))
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeJSON(w, http.StatusUnauthorized, ErrorResponse{
		Code:    "unauthorized",
		Message: msg,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}
